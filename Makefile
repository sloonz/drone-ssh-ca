build:
	docker buildx build -t sloonz/drone-ssh-ca .

publish:
	docker push sloonz/drone-ssh-ca
