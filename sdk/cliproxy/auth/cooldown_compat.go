package auth

func SetQuotaCooldownDisabled(disabled bool) {
	quotaCooldownDisabled.Store(disabled)
}

func isQuotaCooldownDisabled() bool {
	return quotaCooldownDisabled.Load()
}
