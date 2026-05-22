package service

type IBilling interface{}

var localBilling IBilling

func RegisterBilling(i IBilling) { localBilling = i }

func Billing() IBilling {
	if localBilling == nil {
		panic("service.Billing not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localBilling
}
