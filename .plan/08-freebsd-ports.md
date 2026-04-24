# FreeBSD Ports Packaging

## Goal

Submit `bhyve-mcp` to the FreeBSD Ports Collection as `sysutils/bhyve-mcp`.

## Port Structure

```
/usr/ports/sysutils/bhyve-mcp/
├── Makefile
├── distinfo
├── pkg-descr
├── pkg-plist
└── files/
    └── bhyve_mcp.in       # rc.d script template
```

## Makefile

```makefile
PORTNAME=       bhyve-mcp
DISTVERSION=    0.1.0
CATEGORIES=     sysutils

MAINTAINER=     mlapointe@example.com
COMMENT=        MCP server for managing bhyve virtual machines
WWW=            https://github.com/cloudbsdorg/bhyve-mcp

LICENSE=        BSD2CLAUSE

USES=           go:modules
USE_GITHUB=     yes
GH_ACCOUNT=     cloudbsdorg

GO_MODULE=      github.com/cloudbsdorg/bhyve-mcp

PLIST_FILES=    bin/bhyve-mcp \
                etc/rc.d/bhyve_mcp

SUB_LIST=       BHYVE_MCP_USER=${BHYVE_MCP_USER} \
                BHYVE_MCP_GROUP=${BHYVE_MCP_GROUP}

BHYVE_MCP_USER?=        bhyve-mcp
BHYVE_MCP_GROUP?=       bhyve-mcp

USERS=          ${BHYVE_MCP_USER}
GROUPS=         ${BHYVE_MCP_GROUP}

post-install:
        ${MKDIR} ${STAGEDIR}${PREFIX}/etc/bhyve-mcp/vms
        ${MKDIR} ${STAGEDIR}/var/db/bhyve-mcp
        ${MKDIR} ${STAGEDIR}/var/lib/bhyve-mcp/isos
        ${MKDIR} ${STAGEDIR}/var/lib/bhyve-mcp/disks
        ${MKDIR} ${STAGEDIR}/var/lib/bhyve-mcp/templates
        ${MKDIR} ${STAGEDIR}/var/lib/bhyve-mcp/cloud-init
        ${MKDIR} ${STAGEDIR}/var/log/bhyve-mcp/console
        ${INSTALL_SCRIPT} ${WRKSRC}/configs/bhyve_mcp ${STAGEDIR}${PREFIX}/etc/rc.d/bhyve_mcp

.include <bsd.port.mk>
```

## pkg-descr

```
bhyve-mcp is a Model Context Protocol (MCP) server for FreeBSD that
enables AI coding agents (Junie, OpenCode, Claude, etc.) to create,
manage, and monitor bhyve virtual machines directly via the native
libvmmapi C library.

Features:
- VM lifecycle management (create, start, stop, destroy)
- Direct libvmmapi integration (no shell wrappers)
- VNC framebuffer screenshot capture
- Serial console I/O
- ZFS zvol and file-based disk management
- Virtual switch / bridge networking
- FreeBSD rc.d service integration

WWW: https://github.com/cloudbsdorg/bhyve-mcp
```

## pkg-plist

```
bin/bhyve-mcp
etc/rc.d/bhyve_mcp
@dir etc/bhyve-mcp/vms
@dir /var/db/bhyve-mcp
@dir /var/lib/bhyve-mcp/isos
@dir /var/lib/bhyve-mcp/disks
@dir /var/lib/bhyve-mcp/templates
@dir /var/lib/bhyve-mcp/cloud-init
@dir /var/log/bhyve-mcp/console
```

## rc.d Script Template (`files/bhyve_mcp.in`)

```sh
#!/bin/sh
# PROVIDE: bhyve_mcp
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="bhyve_mcp"
rcvar="bhyve_mcp_enable"

load_rc_config $name

: ${bhyve_mcp_enable:="NO"}
: ${bhyve_mcp_user:="%%BHYVE_MCP_USER%%"}
: ${bhyve_mcp_group:="%%BHYVE_MCP_GROUP%%"}
: ${bhyve_mcp_config:="/usr/local/etc/bhyve-mcp/config.yaml"}

pidfile="/var/run/${name}.pid"
command="/usr/local/bin/bhyve-mcp"
command_args="--config ${bhyve_mcp_config}"

run_rc_command "$1"
```

## Port Submission Checklist

- [ ] Port builds with `poudriere` or `make package`
- [ ] `portlint` passes without warnings
- [ ] `make check-plist` passes
- [ ] `make stage-qa` passes
- [ ] Go modules are properly vendored or fetched
- [ ] rc.d script follows FreeBSD style
- [ ] Man page included (if applicable)
- [ ] DESCR is under 80 chars per line
- [ ] LICENSE file included in distfile
- [ ] Upstream tagged release exists

## Testing the Port

```sh
cd /usr/ports/sysutils/bhyve-mcp
make makesum
make
make stage
make check-plist
make package
make install
service bhyve_mcp start
```

## Upstream Release Process

1. Tag release: `git tag v0.1.0`
2. Push tag: `git push origin v0.1.0`
3. Update port `DISTVERSION`
4. Run `make makesum`
5. Submit PR to FreeBSD bugzilla
