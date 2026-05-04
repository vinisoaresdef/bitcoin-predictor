# Predictor Platform - Prerequisites

Document created: 2025-05-04

## Installed Versions

### Go
- **Version**: go1.24.2 linux/amd64
- **Location**: `/home/vinicius-soares/.local/go/go/bin`
- **Install Method**: Downloaded tarball from https://go.dev/dl/
- **PATH Setup**: Add `/home/vinicius-soares/.local/go/go/bin` to PATH

### Docker
- **Version**: Docker version 29.4.0, build 9d7ad9f
- **Type**: Docker Engine (system install)
- **Status**: ✓ Verified

### Docker Compose
- **Version**: Docker Compose version v5.1.3
- **Type**: Docker Compose v2 (plugin)
- **Status**: ✓ Verified (NOT docker-compose v1)

### Python3
- **Version**: Python 3.12.3
- **Type**: System Python (apt install)
- **Status**: ✓ Verified

## PATH Configuration

Add to shell profile (~/.bashrc, ~/.zshrc, etc.):

```bash
export PATH=$PATH:/home/vinicius-soares/.local/go/go/bin
```

## Verification Commands

```bash
go version
docker --version
docker compose version
python3 --version
```

## Notes

- Go was installed to user-local directory to avoid requiring sudo
- Docker Compose v2 is available as `docker compose` (not `docker-compose`)
- All prerequisites meet minimum requirements for the Predictor Platform project