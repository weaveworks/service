package zuora

// StripeError describes a payment error in Stripe.
type StripeError struct {
	Code        StripeDeclineCode
	Description string
	Action      string
}

// StripeErrors lists all known, documented Stripe errors.
// See also: https://stripe.com/docs/declines/codes
var StripeErrors = map[StripeDeclineCode]StripeError{
	ApproveWithID: {
		Code:        ApproveWithID,
		Description: "The payment cannot be authorized.",
		Action:      "Please contact our support team. The payment should be attempted again. If it still cannot be processed, you will need to contact your card issuer.",
	},
	CallIssuer: {
		Code:        CallIssuer,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	CardNotSupported: {
		Code:        CardNotSupported,
		Description: "The card does not support this type of purchase.",
		Action:      "Please contact your card issuer to make sure their card can be used to make this type of purchase.",
	},
	CardVelocityExceeded: {
		Code:        CardVelocityExceeded,
		Description: "The balance or credit limit available on your card has been exceeded.",
		Action:      "Please contact your card issuer for more information, or use another card.",
	},
	CurrencyNotSupported: {
		Code:        CurrencyNotSupported,
		Description: "The card does not support the specified currency.",
		Action:      "Please check with your card issuer that this card can be used for the type of currency specified.",
	},
	DoNotHonor: {
		Code:        DoNotHonor,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	DoNotTryAgain: {
		Code:        DoNotTryAgain,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	DuplicateTransaction: {
		Code:        DuplicateTransaction,
		Description: "A transaction with identical amount and credit card information was submitted very recently.",
		Action:      "Please check to see if a recent payment already exists and contact our support team.",
	},
	ExpiredCard: {
		Code:        ExpiredCard,
		Description: "The card has expired.",
		Action:      "Please use another card.",
	},
	Fraudulent: {
		Code:        Fraudulent,
		Description: "The payment has been declined as Stripe suspects it is fraudulent.",
		Action:      "Do not report more detailed information to your customer. Instead, present as you would the generic_decline described below.",
	},
	GenericDecline: {
		Code:        GenericDecline,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	IncorrectNumber: {
		Code:        IncorrectNumber,
		Description: "The card number is incorrect.",
		Action:      "Please enter your card number again.",
	},
	IncorrectCVC: {
		Code:        IncorrectCVC,
		Description: "The CVC number is incorrect.",
		Action:      "Please enter your card CVC again.",
	},
	IncorrectZip: {
		Code:        IncorrectZip,
		Description: "The ZIP/postal code is incorrect.",
		Action:      "Please enter your card's ZIP/postal code again.",
	},
	InsufficientFunds: {
		Code:        InsufficientFunds,
		Description: "The card has insufficient funds to complete the purchase.",
		Action:      "Please use another card.",
	},
	InvalidAccount: {
		Code:        InvalidAccount,
		Description: "The card, or account the card is connected to, is invalid.",
		Action:      "Please contact your card issuer to check that the card is working correctly.",
	},
	InvalidAmount: {
		Code:        InvalidAmount,
		Description: "The payment amount is invalid, or exceeds the amount that is allowed.",
		Action:      "If the amount appears to be correct, please check with your card issuer that you can make purchases of that amount.",
	},
	InvalidCVC: {
		Code:        InvalidCVC,
		Description: "The CVC number is incorrect.",
		Action:      "Please enter your card CVC again.",
	},
	InvalidExpiryYear: {
		Code:        InvalidExpiryYear,
		Description: "The expiration year is invalid.",
		Action:      "Please enter your card's expiration date again.",
	},
	InvalidNumber: {
		Code:        InvalidNumber,
		Description: "The card number is incorrect.",
		Action:      "Please enter your card's number again.",
	},
	IssuerNotAvailable: {
		Code:        IssuerNotAvailable,
		Description: "The card issuer could not be reached, so the payment could not be authorized.",
		Action:      "Please contact our support team. The payment should be attempted again. If it still cannot be processed, you will need to contact your card issuer.",
	},
	LostCard: {
		Code:        LostCard,
		Description: "The payment has been declined because the card is reported lost.",
		Action:      "The specific reason for the decline should not be reported to the customer. Instead, it needs to be presented as a generic decline.",
	},
	NewAccountInformationAvailable: {
		Code:        NewAccountInformationAvailable,
		Description: "The card, or account the card is connected to, is invalid.",
		Action:      "Please contact your card issuer for more information.",
	},
	NoActionTaken: {
		Code:        NoActionTaken,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	NotPermitted: {
		Code:        NotPermitted,
		Description: "The payment is not permitted.",
		Action:      "Please contact your card issuer for more information.",
	},
	PickupCard: {
		Code:        PickupCard,
		Description: "The card cannot be used to make this payment.",
		Action:      "Please contact your card issuer for more information.",
	},
	ProcessingError: {
		Code:        ProcessingError,
		Description: "An error occurred while processing the card.",
		Action:      "Please contact our support team. The payment should be attempted again.",
	},
	ReenterTransaction: {
		Code:        ReenterTransaction,
		Description: "The payment could not be processed by the issuer for an unknown reason.",
		Action:      "Please contact our support team. The payment should be attempted again. If it still cannot be processed, you will need to contact your card issuer.",
	},
	RestrictedCard: {
		Code:        RestrictedCard,
		Description: "The card cannot be used to make this payment.",
		Action:      "Please contact your card issuer for more information.",
	},
	RevocationOfAllAuthorizations: {
		Code:        RevocationOfAllAuthorizations,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	RevocationOfAuthorization: {
		Code:        RevocationOfAuthorization,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	SecurityViolation: {
		Code:        SecurityViolation,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	ServiceNotAllowed: {
		Code:        ServiceNotAllowed,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	StolenCard: {
		Code:        StolenCard,
		Description: "The payment has been declined because the card is reported stolen.",
		Action:      "The specific reason for the decline should not be reported to the customer. Instead, it needs to be presented as a generic decline.",
	},
	StopPaymentOrder: {
		Code:        StopPaymentOrder,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	TestmodeDecline: {
		Code:        TestmodeDecline,
		Description: "A test card number was used.",
		Action:      "Please use a genuine card to make a payment.",
	},
	TransactionNotAllowed: {
		Code:        TransactionNotAllowed,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact your card issuer for more information.",
	},
	TryAgainLater: {
		Code:        TryAgainLater,
		Description: "The card has been declined for an unknown reason.",
		Action:      "Please contact our support team. The payment should be attempted again. If subsequent payments are declined, you will need to contact your card issuer.",
	},
	WithdrawalCountLimitExceeded: {
		Code:        WithdrawalCountLimitExceeded,
		Description: "The balance or credit limit available on your card has been exceeded.",
		Action:      "Please contact your card issuer for more information, or use another card.",
	},
}

// StripeDeclineCode defines a type alias for Stripe decline codes.
type StripeDeclineCode string

// Known Stripe decline codes:
const (
	ApproveWithID                  = StripeDeclineCode("approve_with_id")
	CallIssuer                     = StripeDeclineCode("call_issuer")
	CardNotSupported               = StripeDeclineCode("card_not_supported")
	CardVelocityExceeded           = StripeDeclineCode("card_velocity_exceeded")
	CurrencyNotSupported           = StripeDeclineCode("currency_not_supported")
	DoNotHonor                     = StripeDeclineCode("do_not_honor")
	DoNotTryAgain                  = StripeDeclineCode("do_not_try_again")
	DuplicateTransaction           = StripeDeclineCode("duplicate_transaction")
	ExpiredCard                    = StripeDeclineCode("expired_card")
	Fraudulent                     = StripeDeclineCode("fraudulent")
	GenericDecline                 = StripeDeclineCode("generic_decline")
	IncorrectNumber                = StripeDeclineCode("incorrect_number")
	IncorrectCVC                   = StripeDeclineCode("incorrect_cvc")
	IncorrectPin                   = StripeDeclineCode("incorrect_pin")
	IncorrectZip                   = StripeDeclineCode("incorrect_zip")
	InsufficientFunds              = StripeDeclineCode("insufficient_funds")
	InvalidAccount                 = StripeDeclineCode("invalid_account")
	InvalidAmount                  = StripeDeclineCode("invalid_amount")
	InvalidCVC                     = StripeDeclineCode("invalid_cvc")
	InvalidExpiryYear              = StripeDeclineCode("invalid_expiry_year")
	InvalidNumber                  = StripeDeclineCode("invalid_number")
	InvalidPin                     = StripeDeclineCode("invalid_pin")
	IssuerNotAvailable             = StripeDeclineCode("issuer_not_available")
	LostCard                       = StripeDeclineCode("lost_card")
	NewAccountInformationAvailable = StripeDeclineCode("new_account_information_available")
	NoActionTaken                  = StripeDeclineCode("no_action_taken")
	NotPermitted                   = StripeDeclineCode("not_permitted")
	PickupCard                     = StripeDeclineCode("pickup_card")
	PinTryExceeded                 = StripeDeclineCode("pin_try_exceeded")
	ProcessingError                = StripeDeclineCode("processing_error")
	ReenterTransaction             = StripeDeclineCode("reenter_transaction")
	RestrictedCard                 = StripeDeclineCode("restricted_card")
	RevocationOfAllAuthorizations  = StripeDeclineCode("revocation_of_all_authorizations")
	RevocationOfAuthorization      = StripeDeclineCode("revocation_of_authorization")
	SecurityViolation              = StripeDeclineCode("security_violation")
	ServiceNotAllowed              = StripeDeclineCode("service_not_allowed")
	StolenCard                     = StripeDeclineCode("stolen_card")
	StopPaymentOrder               = StripeDeclineCode("stop_payment_order")
	TestmodeDecline                = StripeDeclineCode("testmode_decline")
	TransactionNotAllowed          = StripeDeclineCode("transaction_not_allowed")
	TryAgainLater                  = StripeDeclineCode("try_again_later")
	WithdrawalCountLimitExceeded   = StripeDeclineCode("withdrawal_count_limit_exceeded")
)
