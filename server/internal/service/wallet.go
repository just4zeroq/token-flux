package service

type IWallet interface{}

var localWallet IWallet

func RegisterWallet(i IWallet) { localWallet = i }

func Wallet() IWallet {
	if localWallet == nil {
		panic("service.Wallet not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localWallet
}
