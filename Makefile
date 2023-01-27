build:
	docker build -t gmax079/app .

run:
	docker run -d --rm --name app -p8000:8000 gmax079/app

push:
	docker push gmax079/app

