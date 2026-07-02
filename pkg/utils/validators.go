package utils

func ValidateEmail(email string) bool {
	// Simple email validation logic
	return email != "" && len(email) > 5 && email[0] != '@' && email[len(email)-1] != '@'
}
