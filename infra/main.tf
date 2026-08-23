# Secret Configuration
variable "secret_name" {
  description = "Name of the consolidated application secret in AWS Secrets Manager"
  type        = string
  default     = "f1-api/secrets"
}

# Database Credentials
variable "db_username" {
  description = "Database username"
  type        = string
  sensitive   = true
}

variable "db_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}

variable "db_url" {
  description = "Database connection URL"
  type        = string
}

# Google OAuth & Security
variable "google_client" {
  description = "Google OAuth Client ID"
  type        = string
  sensitive   = true
}

variable "google_client_secret" {
  description = "Google OAuth Client Secret"
  type        = string
  sensitive   = true
}

variable "jwt_secret_key" {
  description = "Secret key used to sign JWT tokens"
  type        = string
  sensitive   = true
}

# Upstash Redis Credentials
variable "upstash_redis_token" {
  description = "Upstash Redis REST Token"
  type        = string
  sensitive   = true
}

variable "upstash_redis_url" {
  description = "Upstash Redis REST URL"
  type        = string
}

# AWS & Environment Settings
variable "aws_region" {
  description = "AWS region to deploy resources in"
  type        = string
  default     = "eu-north-1"
}

variable "aws_account_id" {
  description = "Your 12-digit AWS account ID (find it in the AWS Console top-right menu)"
  type        = string
}

variable "env" {
  description = "Deployment environment (e.g. prod, dev)"
  type        = string
  default     = "prod"
}

# GitHub OIDC Configuration
variable "github_org" {
  description = "Your GitHub organization or username"
  type        = string
}

variable "github_repo" {
  description = "Your GitHub repository name"
  type        = string
}

variable "github_org_id" {
  description = "Immutable numeric ID of the GitHub organization (from the OIDC sub claim)"
  type        = string
}

variable "github_repo_id" {
  description = "Immutable numeric ID of the GitHub repository (from the OIDC sub claim)"
  type        = string
}

variable "image_uri" {
  description = "Full ECR image URI (with tag) for the Lambda function, passed from CI"
  type        = string
}


locals {
  ecr_base = "${var.aws_account_id}.dkr.ecr.${var.aws_region}.amazonaws.com"
}
provider "aws" {
  region = var.aws_region
}
# --- GITHUB OIDC IDENTITY PROVIDER ---
data "tls_certificate" "github" {
  url = "https://token.actions.githubusercontent.com"
}
resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.github.certificates[0].sha1_fingerprint]
}
module "ecr" {
  source = "./modules/ecr"
}
module "iam" {
  source             = "./modules/iam"
  aws_account_id     = var.aws_account_id
  github_org         = var.github_org
  github_repo        = var.github_repo
  github_org_id      = var.github_org_id
  github_repo_id     = var.github_repo_id
  ecr_repository_arn = module.ecr.repository_arn
  secret_arn         = module.secrets.secret_arn
}
module "secrets" {
  source               = "./modules/secrets"
  secret_name          = var.secret_name
  db_username          = var.db_username
  db_password          = var.db_password
  db_url               = var.db_url
  google_client        = var.google_client
  google_client_secret = var.google_client_secret
  jwt_secret_key       = var.jwt_secret_key
  upstash_redis_token  = var.upstash_redis_token
  upstash_redis_url    = var.upstash_redis_url
}
module "lambdas" {
  source          = "./modules/lambdas"
  image_uri       = var.image_uri
  lambda_role_arn = module.iam.lambda_role_arn
  env             = var.env
  app_secret_name = module.secrets.secret_name
  jwt_secret_key  = var.jwt_secret_key
}
module "api_gateway" {
  source                  = "./modules/api_gateway"
  lambda_alias_arn        = module.lambdas.lambda_alias_arn
  lambda_alias_invoke_arn = module.lambdas.lambda_alias_invoke_arn
}
terraform {
  backend "s3" {
    bucket         = "f1-api-tfstate"
    key            = "f1-api/terraform.tfstate"
    region         = "eu-north-1"
    dynamodb_table = "f1-api-tf-lock"
    encrypt        = true
  }
}