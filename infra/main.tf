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
  resend_api_key       = var.resend_api_key
  alert_email_from     = var.alert_email_from
  alert_email_to       = var.alert_email_to
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
    use_lockfile = true
    encrypt        = true
  }
}