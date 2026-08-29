ADMIN_API_HOST ?= http://localhost:50006
ADMIN_API_TOKEN ?= changeme

docker-build:
	docker build -f deploy/Dockerfile -t lms-redbus:latest ./

docker-build-admin:
	docker build -f deploy/admin/Dockerfile -t lms-redbus-admin:latest \
		--build-arg apiHost=$(ADMIN_API_HOST) \
		--build-arg apiToken=$(ADMIN_API_TOKEN) ./

docker-run-admin: docker-build-admin
	docker run -d -p 8080:8080 --name lms-redbus-admin lms-redbus-admin:latest
