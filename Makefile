.DEFAULT_GOAL=all

PACKAGES_WITH_TESTS:=$(shell go list -f="{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}" ./... | grep -v '/vendor/')
TEST_TARGETS:=$(foreach p,${PACKAGES_WITH_TESTS},test-$(p))
TMP_DIR:=tmp
TEST_OUT_DIR:=$(TMP_DIR)/testout

PROJECT:= $(subst ${GOPATH}/src/,,$(shell pwd))

.PHONY:all
all: mod linter testall

.PHONY:mod
mod:
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod download

.PHONY: linter
linter:
	docker run -it --rm \
	-v "$(shell pwd):/go/src/${PROJECT}" \
	-v "${GOPATH}/pkg:/go/pkg" \
	-w "/go/src/${PROJECT}" \
	dialogs/go-tools-linter:latest \
	golangci-lint run ./... \
	--exclude "is deprecated" \
	-v

.PHONY:testall
testall:
	mkdir -p -m 755 $(TEST_OUT_DIR)
	$(MAKE) -j 1 $(TEST_TARGETS)
	@echo "=== tests: ok ==="

.PHONY: $(TEST_TARGETS)
$(TEST_TARGETS):
	$(eval $@_package := $(subst test-,,$@))
	$(eval $@_filename := $(subst /,_,$($@_package)))

	@echo "== test directory $($@_package) =="
	@go test $($@_package) -v -race -coverprofile $(TEST_OUT_DIR)/$($@_filename)_cover.out \
	 >> $(TEST_OUT_DIR)/$($@_filename).out \
	 || ( echo 'fail $($@_package)' &&  cat $(TEST_OUT_DIR)/$($@_filename).out; exit 1);

