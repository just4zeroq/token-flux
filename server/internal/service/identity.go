package service

// IIdentity is the contract for the identity domain (users, JWT, API keys).
// Methods will be filled in during the identity migration plan; this skeleton
// only establishes the registration mechanism.
type IIdentity interface{}

var localIdentity IIdentity

// RegisterIdentity installs the identity implementation. Called once during
// init() from internal/logic/identity.
func RegisterIdentity(i IIdentity) { localIdentity = i }

// Identity returns the registered identity service. Panics if no implementation
// has been registered, which indicates a missing blank import of internal/logic.
func Identity() IIdentity {
	if localIdentity == nil {
		panic("service.Identity not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localIdentity
}
