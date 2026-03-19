package auth

func getSafeStoragePasswordsOS(_ string) []string {
	// Windows uses DPAPI, not password-based encryption.
	return nil
}
