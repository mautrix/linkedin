package connector

import (
	"go.mau.fi/util/l10n"
)

var L = l10n.NewNamespace("mautrix-linkedin")

var (
	msgFlowCookiesName         = L.Define("login.flow.cookies.name", "Cookies")
	msgFlowCookiesDescription  = L.Define("login.flow.cookies.description", "Log in using cookies from linkedin.com")
	msgFlowPasswordName        = L.Define("login.flow.password.name", "Email and password")
	msgFlowPasswordDescription = L.Define("login.flow.password.description", "Log in using your email/phone and password")

	msgFieldIdentifierName         = L.Define("login.field.identifier.name", "Email or phone number")
	msgFieldPasswordName           = L.Define("login.field.password.name", "Password")
	msgFieldEmailCodeName          = L.Define("login.field.email_code.name", "Email verification code")
	msgFieldSMSCodeName            = L.Define("login.field.sms_code.name", "SMS verification code")
	msgFieldTOTPCodeName           = L.Define("login.field.totp_code.name", "Authenticator app code")
	msgFieldVerificationMethodName = L.Define("login.field.verification_method.name", "Verification method")
	msgOptionApproveInApp          = L.Define("login.field.verification_method.option.app", "Approve in LinkedIn app")
	msgOptionTryAnotherWay         = L.Define("login.field.verification_method.option.another_way", "Try another way")
	msgFieldSignInMethodName       = L.Define("login.field.sign_in_method.name", "Sign-in method")
	msgOptionContinueInBrowser     = L.Define("login.field.sign_in_method.option.browser", "Continue in browser")
	msgOptionTryPasswordAgain      = L.Define("login.field.sign_in_method.option.password", "Try email and password again")

	msgStepCookiesInstructions = L.Define(
		"login.step.cookies.instructions",
		"Enter a JSON object with your cookies, or a cURL command copied from browser devtools. It is recommended that you use a tab opened in Incognito/Private browsing mode and close the browser **before** pasting the cookies.",
	)
	msgStepBrowserInstructions             = L.Define("login.step.browser.instructions", "Finish signing in to LinkedIn in the browser, including any verification it requests.")
	msgStepCredentialsInstructions         = L.Define("login.step.credentials.instructions", "Enter your LinkedIn email or phone number and password.")
	msgStepCredentialsMissingInstructions  = L.Define("login.step.credentials_missing.instructions", "Enter both your LinkedIn email or phone number and password.")
	msgStepCredentialsRejectedInstructions = L.Define("login.step.credentials_rejected.instructions", "LinkedIn rejected the sign-in. Check your details and try again, or choose the Cookies login method to sign in through a browser.")
	msgStepEmailCodeInstructions           = L.Define("login.step.email_code.instructions", "LinkedIn sent a verification code to your email address. Enter the six-digit code to finish signing in.")
	msgStepEmailCodeRejectedInstructions   = L.Define("login.step.email_code_rejected.instructions", "LinkedIn did not accept that code. Check the latest verification email and try again.")
	msgStepSMSCodeInstructions             = L.Define("login.step.sms_code.instructions", "LinkedIn sent a text message to your phone. Enter the six-digit code to finish signing in.")
	msgStepSMSCodeRejectedInstructions     = L.Define("login.step.sms_code_rejected.instructions", "LinkedIn did not accept that code. Check the latest text message and try again.")
	msgStepTOTPCodeInstructions            = L.Define("login.step.totp_code.instructions", "Enter the current six-digit LinkedIn code from your authenticator app to finish signing in.")
	msgStepTOTPCodeRejectedInstructions    = L.Define("login.step.totp_code_rejected.instructions", "LinkedIn did not accept that code. Enter the current six-digit code from your authenticator app and try again.")
	msgStepAppMethodInstructions           = L.Define("login.step.app_method.instructions", "LinkedIn sent a sign-in request to your LinkedIn app. Approve it there, or choose another verification method.")
	msgStepAppApprovalInstructions         = L.Define("login.step.app_approval.instructions", "Open the LinkedIn app on your phone and approve this sign-in. To use another verification method, go back.")
	msgStepFallbackInstructions            = L.Define("login.step.fallback.instructions", "Choose how to continue signing in.")
	msgStepFallbackFailedInstructions      = L.Define("login.step.fallback_failed.instructions", "LinkedIn could not complete this sign-in directly. You can finish signing in through the browser.")
	msgStepFallbackAppExpiredInstructions  = L.Define("login.step.fallback_app_expired.instructions", "The LinkedIn app approval request expired. Try signing in again or continue through the browser.")
	msgStepFallbackVerifiedInstructions    = L.Define("login.step.fallback_verified.instructions", "LinkedIn verified your sign-in but could not finish logging in. Continue through the browser.")
	msgStepFallbackChallengeInstructions   = L.Define("login.step.fallback_challenge.instructions", "LinkedIn requires additional verification. Continue signing in through the browser.")
	msgStepCompleteInstructions            = L.Define(
		"login.step.complete.instructions", "Successfully logged in as {name}",
		l10n.WithComment("name is the LinkedIn account's first and last name"),
	)
)
