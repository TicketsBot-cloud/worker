package logic

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/member"
	"github.com/TicketsBot-cloud/gdl/objects/user"
	"github.com/TicketsBot-cloud/worker"
)

type SubstitutionFunc func(user user.User, member member.Member) string

type Substitutor struct {
	Placeholder string
	NeedsUser   bool
	NeedsMember bool
	F           SubstitutionFunc
}

func NewSubstitutor(placeholder string, needsUser, needsMember bool, f SubstitutionFunc) Substitutor {
	return Substitutor{
		Placeholder: placeholder,
		NeedsUser:   needsUser,
		NeedsMember: needsMember,
		F:           f,
	}
}

func DoSubstitutions(worker *worker.Context, s string, userId uint64, guildId uint64, substitutors []Substitutor) (string, error) {
	var needsUser, needsMember bool

	// Determine which objects we need to fetch
	for _, substitutor := range substitutors {
		if substitutor.NeedsUser {
			needsUser = true
		}

		if substitutor.NeedsMember {
			needsMember = true
		}

		if needsUser && needsMember {
			break
		}
	}

	// Retrieve user and member if necessary
	var user user.User
	var member member.Member

	var err error
	if needsUser {
		user, err = worker.GetUser(userId)
	}

	if err != nil {
		return "", err
	}

	if needsMember {
		member, err = worker.GetGuildMember(guildId, userId)
	}

	if err != nil {
		return "", err
	}

	for _, substitutor := range substitutors {
		placeholder := fmt.Sprintf("%%%s%%", substitutor.Placeholder)

		if strings.Contains(s, placeholder) {
			s = strings.ReplaceAll(s, placeholder, substitutor.F(user, member))
		}
	}

	return s, nil
}

// ParameterizedSubstitutionFunc handles placeholders with optional parameters for naming schemes
type ParameterizedSubstitutionFunc func(user user.User, member member.Member, params []string) string

// ParameterizedSubstitutor handles placeholders with optional parameters for naming schemes
type ParameterizedSubstitutor struct {
	BaseName    string
	NeedsUser   bool
	NeedsMember bool
	F           ParameterizedSubstitutionFunc
}

func NewParameterizedSubstitutor(baseName string, needsUser, needsMember bool, f ParameterizedSubstitutionFunc) ParameterizedSubstitutor {
	return ParameterizedSubstitutor{
		BaseName:    baseName,
		NeedsUser:   needsUser,
		NeedsMember: needsMember,
		F:           f,
	}
}

// namingParamPlaceholderRegex matches %name% or %name:params%
var namingParamPlaceholderRegex = regexp.MustCompile(`%([a-z_]+)(?::([^%]+))?%`)

// DoSubstitutionsWithParams processes both simple and parameterized substitutions for naming schemes
func DoSubstitutionsWithParams(
	worker *worker.Context,
	s string,
	userId uint64,
	guildId uint64,
	substitutors []Substitutor,
	paramSubstitutors []ParameterizedSubstitutor,
) (string, error) {
	// First, handle parameterized substitutions
	matches := namingParamPlaceholderRegex.FindAllStringSubmatch(s, -1)

	// Determine if we need user/member for parameterized substitutors
	var needsUserParam, needsMemberParam bool
	for _, match := range matches {
		baseName := match[1]
		for _, ps := range paramSubstitutors {
			if ps.BaseName == baseName {
				if ps.NeedsUser {
					needsUserParam = true
				}
				if ps.NeedsMember {
					needsMemberParam = true
				}
				break
			}
		}
	}

	// Get user/member if needed for parameterized substitutors
	var userObj user.User
	var memberObj member.Member
	var err error

	if needsUserParam {
		userObj, err = worker.GetUser(userId)
		if err != nil {
			return "", err
		}
	}

	if needsMemberParam {
		memberObj, err = worker.GetGuildMember(guildId, userId)
		if err != nil {
			return "", err
		}
	}

	// Process parameterized substitutions
	for _, match := range matches {
		fullMatch := match[0]
		baseName := match[1]
		paramStr := ""
		if len(match) > 2 {
			paramStr = match[2]
		}

		// Find matching parameterized substitutor
		for _, ps := range paramSubstitutors {
			if ps.BaseName == baseName {
				var params []string
				if paramStr != "" {
					params = strings.Split(paramStr, ":")
				}

				replacement := ps.F(userObj, memberObj, params)
				s = strings.Replace(s, fullMatch, replacement, 1)
				break
			}
		}
	}

	// Then handle simple substitutions (existing behavior)
	return DoSubstitutions(worker, s, userId, guildId, substitutors)
}
