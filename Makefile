.PHONY: license
license: ## Add license header to all files.
	docker run -it -v ${PWD}:/src ghcr.io/google/addlicense -l apache  -c  "The Kubegems Authors" .