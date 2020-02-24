.PHONY: test
test:
	-go test -covermode=count -coverprofile=./zdevelop/tests/zreports/coverage.out -coverpkg=./... ./...
	-go tool cover -html=./zdevelop/tests/zreports/coverage.out
	open ./zdevelop/tests/zreports/coverage.html

.PHONY: format
format:
	-gofmt -s -w .

.PHONY: venv
venv:
ifeq ($(py), )
	$(eval PY_PATH := python3)
else
	$(eval PY_PATH := $(py))
endif
	$(eval VENV_PATH := $(shell $(PY_PATH) ./zdevelop/make_scripts/make_venv.py))
	@echo "venv created! To enter virtual env, run:"
	@echo ". ~/.bash_profile"
	@echo "then run:"
	@echo "$(VENV_PATH)"

.PHONY: install-dev
install-dev:
	pip install --upgrade pip
	pip install -r requirements.txt
