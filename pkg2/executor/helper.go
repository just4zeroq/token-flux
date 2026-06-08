package executor

import (
	"time"

	"ai-platform/pkg/translator"
)

// secondsAsDuration converts an int seconds to time.Duration.
func secondsAsDuration(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

// shouldPassthrough checks if inbound format matches one of executor's native formats.
// When true, TransformRequest should skip format conversion — body passes through.
// When false, inbound body needs conversion to executor's native format.
func shouldPassthrough(info *RequestInfo, nativeFormats ...translator.Format) bool {
	if info.InboundFormat == "" {
		return true // no format specified, assume native
	}
	for _, f := range nativeFormats {
		if info.InboundFormat == f {
			return true
		}
	}
	return false
}
