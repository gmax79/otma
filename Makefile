build:
	docker build -f app/Dockerfile -t gmax079/app:v3 .
	docker build -f auth/Dockerfile -t gmax079/auth:v1 .

build2:
	cd files && docker build -t gmax079/gateway:v1 .

push:
	docker push gmax079/app:v3
	docker push gmax079/auth:v1

push2:
	docker push gmax079/gateway:v1

local-run:
	docker run --network host --rm -d --name app -v ./config:/app/config -v ./secret:/app/secret gmax079/app:v3
	docker run --network host --rm -d --name auth -v ./config:/app/config -v ./secret:/app/secret gmax079/auth:v1

local-stop:
	docker stop app auth

local-env-up:
	docker-compose -f local_deploy/docker-compose.yml up

local-env-down:
	docker-compose -f local_deploy/docker-compose.yml down

local-test:
	newman run --verbose --environment=tests/dockercompose_env.json tests/postman.json

test:
	newman run --verbose --environment=tests/kuber_env.json tests/postman.json

deploy:
	helm upgrade --install --namespace=m --create-namespace \
    --values ./helm/values.yaml \
    test ./helm/

undeploy:
	helm uninstall test --namespace m

install-ingress:
	kubectl create namespace m && helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx/ && helm repo update && helm install nginx ingress-nginx/ingress-nginx --namespace m -f files/nginx-ingress.yml

postgres-port-forwarding:
	kubectl --namespace m port-forward service/postgres 5432:5432 --address 0.0.0.0

