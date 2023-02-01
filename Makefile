build:
	docker build -t gmax079/app:v1 .

run:
	docker run -d --rm --name app -p8000:8000 gmax079/app:v1

push:
	docker push gmax079/app:v1

deploy:
	 cd helm && kubectl --namespace m apply -f .

undeploy:
	cd helm && kubectl --namespace m delete -f .

install-ingress:
	kubectl create namespace m && helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx/ && helm repo update && helm install nginx ingress-nginx/ingress-nginx --namespace m -f config/nginx-ingress.yaml

test-service-port-forwarding:
	kubectl --namespace m port-forward service/test 8000:8000 --address 0.0.0.0

test:
	newman run --verbose tests/postman.json

curl-test:
	curl -w "\n" http://arch.homework/health
	curl -w "\n" http://arch.homework/otusapp/maksim/request
	curl -w "\n" http://arch.homework/some/url/request
