package identity

import "ai-platform/internal/service"

type sIdentity struct{}

func init() { service.RegisterIdentity(New()) }

// New returns the identity domain implementation. Empty in the skeleton;
// methods are added during the identity migration plan.
func New() *sIdentity { return &sIdentity{} }
