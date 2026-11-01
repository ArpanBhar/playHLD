package identity

import "testing"

func TestPasswordHashAndVerification(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(hash, "incorrect password") {
		t.Fatal("incorrect password accepted")
	}
	if VerifyPassword("argon2id$v=19$m=999999999,t=3,p=2$bad$bad", "anything") {
		t.Fatal("unsafe parameters accepted")
	}
}

func TestEmailNormalization(t *testing.T) {
	email, err := NormalizeEmail("  USER@Example.COM  ")
	if err != nil || email != "user@example.com" {
		t.Fatalf("unexpected email %q: %v", email, err)
	}
	if _, err := NormalizeEmail("not an email"); err == nil {
		t.Fatal("invalid email accepted")
	}
}
