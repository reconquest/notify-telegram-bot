NAME = $(notdir $(shell pwd))

VERSION = $(shell printf "%s.%s" \
	$$(git rev-list --count HEAD) \
	$$(git rev-parse --short HEAD) \
)

# could be "..."
TARGET =

GOFLAGS = GO111MODULE=on CGO_ENABLED=0

version:
	@echo $(VERSION)

test:
	$(GOFLAGS) go test -failfast -v ./$(TARGET)

get:
	$(GOFLAGS) go get -v -d

build:
	$(GOFLAGS) go build \
		 -ldflags="-s -w -X main.version=$(VERSION)" \
		 -gcflags="-trimpath=$(GOPATH)" \
		 ./$(TARGET)

image:
	@echo :: building image $(NAME):$(VERSION)
	@docker build -t $(NAME):$(VERSION) -f Dockerfile .

push:
	$(if $(REMOTE),,$(error REMOTE is not set))
	$(eval VERSION ?= latest)
	@echo :: pushing image $(TAG)
	docker tag $(NAME):$(VERSION) $(REMOTE)/$(NAME):$(VERSION)
	docker push $(REMOTE)/$(NAME):$(VERSION)
	docker tag $(NAME):$(VERSION) $(REMOTE)/$(NAME):latest
	docker push $(REMOTE)/$(NAME):latest

all: build image push
