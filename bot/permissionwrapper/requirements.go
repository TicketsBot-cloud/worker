package permissionwrapper

import (
	"github.com/TicketsBot-cloud/common/botpermissions"
	"github.com/TicketsBot-cloud/gdl/permission"
)

// Adds EmbedLinks, which upstream omits although close.go posts an embed to the
// archive channel. Drop once common/botpermissions covers it.
var TranscriptChannelRequired = append(
	append([]permission.Permission{}, botpermissions.TranscriptChannelRequired...),
	permission.EmbedLinks,
)
