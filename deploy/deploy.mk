docker-build:
	docker build -f deploy/Dockerfile -t lms-redbus:latest ./

docker-build-admin:
	docker build -f deploy/admin/Dockerfile -t lms-redbus-admin:latest ./

docker-run-admin: docker-build-admin
	docker run -d -p 8080:80 --name lms-redbus-admin lms-redbus-admin:latest