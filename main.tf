provider "aws" {
  region = "eu-central-1"
}

# terraform import aws_cognito_user_pool.platform_users_nonprod eu-central-1_C6bwuggLI
resource "aws_cognito_user_pool" "platform_users_nonprod" {
  name                = "platform-users-nonprod"
  deletion_protection = "INACTIVE"

  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]

  sms_authentication_message = "Your authentication code is {####}."
  mfa_configuration          = "OPTIONAL"

  username_configuration {
    case_sensitive = false
  }

  user_pool_add_ons {
    advanced_security_mode = "AUDIT"
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }

    recovery_mechanism {
      name     = "verified_phone_number"
      priority = 2
    }
  }

  password_policy {
    minimum_length                   = 8
    require_lowercase                = true
    require_numbers                  = true
    require_symbols                  = true
    require_uppercase                = true
    temporary_password_validity_days = 7
  }

  user_attribute_update_settings {
    attributes_require_verification_before_update = []
  }

  tags = {
    "Resource-Type" = "nonprod"
  }
}
