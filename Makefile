build:
	docker build -t gmax079/app:v2 .

run:
	docker run -d --rm --name app -p8000:8000 gmax079/app:v2

push:
	docker push gmax079/app:v2

install-ingress:
	kubectl create namespace m && helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx/ && helm repo update && helm install nginx ingress-nginx/ingress-nginx --namespace m -f files/nginx-ingress.yml

test-service-port-forwarding:
	kubectl --namespace m port-forward service/postgres 5432:5432 --address 0.0.0.0

test:
	newman run --verbose tests/postman.json

local-env-up:
	docker-compose -f local_deploy/docker-compose.yml up

local-env-down:
	docker-compose -f local_deploy/docker-compose.yml down

deploy:
	helm upgrade --install --namespace=m --create-namespace \
    --values ./helm/values.yaml \
    test ./helm/

undeploy:
	helm uninstall test --namespace m

