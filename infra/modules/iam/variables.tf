variable "aws_account_id" {
  description = "AWS account ID"
  type        = string
}

variable "github_org" {
  description = "GitHub organization or user that owns the repo"
  type        = string
}

variable "github_repo" {
  description = "GitHub repository name"
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

variable "ecr_repository_arn" {
  description = "ARN of the ECR repository the Lambda role needs pull access to"
  type        = string
}

variable "secret_arn" {
  description = "ARN of the Secrets Manager secret the Lambda role needs read access to"
  type        = string
}