SHELL := sh

TF_DIR ?= deploy
TFVARS ?= deploy/dev.tfvars
TFVARS_FILE = $(notdir $(TFVARS))
AWS_REGION ?= us-east-2
IMAGE_TAG ?= latest

.PHONY: help bootstrap push-image deploy health destroy

help:
	@echo "make bootstrap  Create the base AWS infrastructure and ECR repository (first deployment only)"
	@echo "make deploy     Build and push the API image, then create/update the ECS service"
	@echo "make health     Call the deployed API health endpoint"
	@echo "make destroy    Destroy the Terraform environment"
	@echo "Override defaults with TFVARS=deploy/other.tfvars AWS_REGION=us-east-2 IMAGE_TAG=my-tag"

bootstrap:
	@if [ ! -f "$(TFVARS)" ]; then cp "$(TFVARS).example" "$(TFVARS)"; echo "Created $(TFVARS) from its example."; fi
	terraform -chdir=$(TF_DIR) init
	terraform -chdir=$(TF_DIR) validate
	@if terraform -chdir=$(TF_DIR) output -raw ecr_repository_url >/dev/null 2>&1; then echo "Infrastructure already exists; use 'make deploy'."; exit 1; fi
	terraform -chdir=$(TF_DIR) apply -var-file=$(TFVARS_FILE) -var="container_image="

push-image:
	@test -f "$(TFVARS)" || { echo "Missing $(TFVARS); run 'make bootstrap' first."; exit 1; }
	@set -eu; \
	API_REPOSITORY="$$(terraform -chdir=$(TF_DIR) output -raw ecr_repository_url)"; \
	if [ -z "$$API_REPOSITORY" ]; then echo "ECR output not found; run 'make bootstrap' first."; exit 1; fi; \
	REGISTRY="$$(printf '%s' "$$API_REPOSITORY" | cut -d/ -f1)"; \
	IMAGE_URI="$$API_REPOSITORY:$(IMAGE_TAG)"; \
	aws ecr get-login-password --region "$(AWS_REGION)" | docker login --username AWS --password-stdin "$$REGISTRY"; \
	docker build -t "cards-api:$(IMAGE_TAG)" .; \
	docker tag "cards-api:$(IMAGE_TAG)" "$$IMAGE_URI"; \
	docker push "$$IMAGE_URI"

deploy: push-image
	@set -eu; \
	API_REPOSITORY="$$(terraform -chdir=$(TF_DIR) output -raw ecr_repository_url)"; \
	IMAGE_URI="$$API_REPOSITORY:$(IMAGE_TAG)"; \
	terraform -chdir=$(TF_DIR) apply -var-file=$(TFVARS_FILE) -var="container_image=$$IMAGE_URI"; \
	CLUSTER="$$(terraform -chdir=$(TF_DIR) output -raw ecs_cluster_name)"; \
	SERVICE="$$(terraform -chdir=$(TF_DIR) output -raw ecs_service_name)"; \
	aws ecs update-service --region "$(AWS_REGION)" --cluster "$$CLUSTER" --service "$$SERVICE" --force-new-deployment >/dev/null; \
	aws ecs wait services-stable --region "$(AWS_REGION)" --cluster "$$CLUSTER" --services "$$SERVICE"

health:
	@set -eu; API_URL="$$(terraform -chdir=$(TF_DIR) output -raw api_url)"; curl -k "$$API_URL/health"

destroy:
	terraform -chdir=$(TF_DIR) destroy -var-file=$(TFVARS_FILE)
