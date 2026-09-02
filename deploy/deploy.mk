ADMIN_API_HOST ?=
ADMIN_API_TOKEN ?= changeme

docker-build:
	docker build -f deploy/Dockerfile -t lms-redbus:latest ./

docker-build-admin:
	docker build -f deploy/admin/Dockerfile -t lms-redbus-admin:latest \
		--build-arg apiToken=$(ADMIN_API_TOKEN) ./

docker-run-admin: docker-build-admin
	@test -n "$(ADMIN_API_HOST)" || { echo "ADMIN_API_HOST is required" >&2; exit 1; }
	docker run -d -p 8080:8080 --name lms-redbus-admin \
		-e REDBUS_API_HOST=$(ADMIN_API_HOST) lms-redbus-admin:latest
