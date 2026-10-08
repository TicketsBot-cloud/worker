package logic

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AccessControlOutcome int

const (
	// Zero value, so an uninitialised outcome fails closed.
	AccessControlDeniedInvalid AccessControlOutcome = iota
	AccessControlDeniedByRule
	AccessControlDeniedNotAllowListed
	AccessControlAllowedByRule
	AccessControlAllowedNoRestriction
)

func (o AccessControlOutcome) Allowed() bool {
	return o == AccessControlAllowedByRule || o == AccessControlAllowedNoRestriction
}

// EvaluateAccessControl applies an ordered rule list to a member. rules must be in ascending
// position order, which GetAll guarantees. everyoneRoleId is the guild id: Discord omits @everyone
// from member.Roles.
func EvaluateAccessControl(
	rules []database.PanelAccessControlRule,
	memberRoles []uint64,
	everyoneRoleId uint64,
) (AccessControlOutcome, *database.PanelAccessControlRule) {
	held := make(map[uint64]struct{}, len(memberRoles)+1)
	held[everyoneRoleId] = struct{}{}
	for _, id := range memberRoles {
		held[id] = struct{}{}
	}

	// A list granting nothing to anybody is a blocklist, so its terminal default is allow.
	// Testing for an exact deny (rather than the absence of an exact allow) makes a corrupt
	// action read as a grant, so the list stays a whitelist and an unmatched member is denied.
	blocklistOnly := true

	for i := range rules {
		if rules[i].Action != database.AccessControlActionDeny {
			blocklistOnly = false
		}

		if _, ok := held[rules[i].RoleId]; !ok {
			continue
		}

		if rules[i].Action == database.AccessControlActionAllow {
			return AccessControlAllowedByRule, &rules[i]
		}

		return AccessControlDeniedByRule, &rules[i]
	}

	if blocklistOnly {
		return AccessControlAllowedNoRestriction, nil
	}

	return AccessControlDeniedNotAllowListed, nil
}

type aclDenial struct {
	Content i18n.MessageId
	Args    []any
}

func buildAclDenial(
	rules []database.PanelAccessControlRule,
	matched *database.PanelAccessControlRule,
	everyoneRoleId uint64,
) aclDenial {
	// Only reached on denial, so a non-@everyone match is always a deny rule.
	if matched != nil && matched.RoleId != everyoneRoleId {
		return aclDenial{Content: i18n.MessageOpenAclDenyListed, Args: []any{matched.RoleId}}
	}

	allowedRoleIds := make([]uint64, 0, len(rules))
	for _, rule := range rules {
		if rule.Action == database.AccessControlActionAllow {
			allowedRoleIds = append(allowedRoleIds, rule.RoleId)
		}
	}

	if len(allowedRoleIds) == 0 {
		return aclDenial{Content: i18n.MessageOpenAclNoAllowRules}
	}

	// No match, or matched only via @everyone: both read as "you are not on the list".
	mentions := make([]string, 0, len(allowedRoleIds))
	for _, roleId := range allowedRoleIds {
		mentions = append(mentions, fmt.Sprintf("<@&%d>", roleId))
	}

	content := i18n.MessageOpenAclNotAllowListedMultiple
	if len(allowedRoleIds) == 1 {
		content = i18n.MessageOpenAclNotAllowListedSingle
	}

	return aclDenial{Content: content, Args: []any{strings.Join(mentions, ", ")}}
}
