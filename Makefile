# bhyve-mcp Makefile - Compatible with FreeBSD make
# Supports: build, test, install, upgrade, clean

# Project variables
PROG=		bhyve-mcp
SRCDIR=		${.CURDIR}
BINDIR=		${.CURDIR}/bin
GO=		go
GOCMD=		${GO}
GOFLAGS=	-v
CGO_CFLAGS=	-I/usr/include
CGO_LDFLAGS=	-L/usr/lib -lvmmapi

# Go build variables
GOPATH=		${HOME}/go
GOBIN=		${GOPATH}/bin
CGO_ENABLED=	1

# Build targets
.PATH:		${SRCDIR}

all: build

build:
	${GOCMD} build ${GOFLAGS} -o ${BINDIR}/${PROG} ./cmd/${PROG}

# Alternative build target for system installation
build-system:
	${GOCMD} build ${GOFLAGS} -o /usr/local/bin/${PROG} ./cmd/${PROG}

# Test targets
test:
	${GOCMD} test ${GOFLAGS} ./...

test-verbose:
	${GOCMD} test ${GOFLAGS} -v ./...

test-cover:
	${GOCMD} test ${GOFLAGS} -cover ./...

# Install targets
install: build
	install -o root -g wheel -m 755 ${BINDIR}/${PROG} /usr/local/bin/

install-local: build
	${GOCMD} install ./cmd/${PROG}

# Upgrade target - pulls latest changes and rebuilds
upgrade:
	${GOCMD} get -u ./...
	${GOCMD} mod tidy
	${GOCMD} build ${GOFLAGS} -o ${BINDIR}/${PROG} ./cmd/${PROG}

# Module management
mod-init:
	${GOCMD} mod init

mod-tidy:
	${GOCMD} mod tidy

mod-vendor:
	${GOCMD} mod vendor

# Clean targets
clean:
	rm -rf ${BINDIR}
	rm -f ${PROG}
	${GOCMD} clean ./...

distclean: clean
	rm -rf vendor
	${GOCMD} mod tidy

# Linting and formatting
fmt:
	${GOCMD} fmt ./...

vet:
	${GOCMD} vet ./...

lint: fmt vet

# Run targets
run: build
	${BINDIR}/${PROG}

run-sudo: build
	sudo ${BINDIR}/${PROG}

# Debug build
debug: GOFLAGS += -gcflags="all=-N -l"
debug: build

# Static analysis
check: lint test

# Help target
help:
	@echo "Available targets:"
	@echo "  all          - Build the project (default)"
	@echo "  build        - Compile bhyve-mcp binary"
	@echo "  build-system - Build and install to /usr/local/bin"
	@echo "  test         - Run all tests"
	@echo "  test-verbose - Run tests with verbose output"
	@echo "  test-cover   - Run tests with coverage report"
	@echo "  install      - Build and install to /usr/local/bin"
	@echo "  install-local- Build and install to GOPATH/bin"
	@echo "  upgrade      - Update dependencies and rebuild"
	@echo "  mod-init     - Initialize Go module"
	@echo "  mod-tidy     - Tidy Go module dependencies"
	@echo "  mod-vendor   - Vendor Go dependencies"
	@echo "  clean        - Remove build artifacts"
	@echo "  distclean    - Remove build artifacts and vendor"
	@echo "  fmt          - Format Go source files"
	@echo "  vet          - Run Go vet"
	@echo "  lint         - Format and vet source files"
	@echo "  run          - Build and run locally"
	@echo "  run-sudo     - Build and run with sudo"
	@echo "  debug        - Build with debug symbols"
	@echo "  check        - Run lint and tests"
	@echo "  help         - Show this help message"

.PHONY: all build build-system test test-verbose test-cover install install-local \
	upgrade mod-init mod-tidy mod-vendor clean distclean fmt vet lint run run-sudo \
	debug check help
