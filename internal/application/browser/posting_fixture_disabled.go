//go:build !desktop || !darwin || !qualification

package browser

func isNativeFillFixture(string) bool { return false }
