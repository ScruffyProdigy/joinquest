package auth

import "errors"

var (
	ErrAuthenticationRequired    = errors.New("authentication required")
	ErrGuestAccountNotAllowed    = errors.New("Add an email in Account settings to use developer tools and save purchases.")
	ErrEmailAlreadyLinked        = errors.New("That email is already linked to another account.")
	ErrLastSignInMethod          = errors.New("Keep at least one sign-in method on your account.")
	ErrEmailNotLinked            = errors.New("That email is not linked to your account.")
	ErrMergeConfirmationRequired = errors.New("This email belongs to another JoinQuest account. Confirm the merge to continue.")
	ErrIdentityNotLinked         = errors.New("That connected account is not linked to your profile.")
)

type EmailLinkPreview struct {
	Email                  string
	WillMergeAccounts      bool
	MergeSourceDisplayName string
}
