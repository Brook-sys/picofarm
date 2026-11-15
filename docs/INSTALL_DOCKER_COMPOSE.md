# Install PicoFarm with Docker Compose

PicoFarm is open source and in **BETA**. It began as a fork of [Daedalus](https://github.com/philjestin/Daedalus); development of this fork is entirely AI-driven (vibe coding). Expect bugs, supervise physical operations and keep recoverable backups.

Docker Compose is the recommended end-user installation path. Source development instructions remain in [Getting started](GETTING_STARTED.md). A packaged installation without Docker is a future task, not an available installation option.

## 1. Prepare your server

Install Git, Docker Engine and the Docker Compose plugin using your operating system's official instructions. The server must be able to reach your printers. Confirm:

```sh
docker --version
docker compose version
```

No hardware-specific CPU/RAM minimum has been established. Fatiamento/slicing is an optional separate service and can have much higher resource requirements. The public website does not host your printers, data or PicoFarm instance.

## 2. Download and configure

```sh
git clone https://github.com/Brook-sys/picofarm.git
cd picofarm
cp .env.compose.example .env
openssl rand -hex 32
```

Copy the generated random value into `ENCRYPTION_KEY` in `.env`, using a text editor. Keep that file private. Do not paste the key into an Issue. **Retain the same key across upgrades and restores**; replacing it can make saved encrypted integration settings unreadable. Compose refuses to start with an empty key. If OpenSSL is unavailable, generate a long random secret with a trusted password manager.

The included `compose.yaml` uses a named volume for `/data` and binds the application to `127.0.0.1:8084` only. It does not publish the app to your LAN or the internet. Docker privileges are powerful; do not grant them to untrusted accounts.

## 3. Start and check

```sh
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 picofarm
curl --fail http://localhost:8084/health
```

Open `http://localhost:8084` **on the server**. For a remote server, an SSH tunnel avoids changing the default exposure:

```sh
ssh -L 8084:127.0.0.1:8084 YOUR-USER@YOUR-SERVER
```

Then open `http://localhost:8084` on your computer. A successful `/health` means the process responds; it does not validate printers or finished parts.

For LAN/reverse-proxy access, deliberately configure the published address and `ALLOWED_ORIGINS` for the browser's actual origin. CORS is not authentication. Configure API authentication and transport protection before allowing untrusted access; read [Security model](SECURITY_MODEL.md). Do not port-forward an unauthenticated instance to the internet.

## 4. Try your workflow

Start by exploring the STL/G-code library, filament inventory and queue. Add one printer and verify connectivity, status and camera support before sending a supervised test job. Availability of controls, cameras, files and consumption information varies by printer/protocol. Reported filament consumption is an estimate, not a physical measurement.

Online slicing requires an independently configured compatible slicer service. This minimal Compose file does not install it. Do not expose that service directly without its own access protection. See [slicer jobs](SLICER_JOBS.md).

Semi-automatic printing and commercial workflows (quotes, customers, projects, tasks, costs, sales and marketplace order synchronization) are **experimental, for testing only**. A visible provider entry is not proof that account connectivity or order import works. See [sales channels](SALES_CHANNELS.md) for actual provider boundaries.

## 5. Keep and update your data

The named volume contains the database, uploads and database backups. The `.env` encryption key must be preserved separately and securely. Database backups alone do not back up uploaded files. Test restoration before relying on a backup.

Before upgrading: finish or safely pause physical work, back up data and key, record the running image digest and read changes. `latest` is mutable; use a tested digest in `compose.yaml` if you need a reproducible version.

```sh
docker compose pull
docker compose up -d
docker compose ps
curl --fail http://localhost:8084/health
```

Startup can migrate the database: switching to an old image is not necessarily a safe database rollback. Do not run two instances against the same data volume.

```sh
docker compose down
```

This stops the app without deleting the named volume. **Do not run `docker compose down -v` unless you intentionally want to delete persistent data.**

## 6. Report a problem

Use [GitHub Issues](https://github.com/Brook-sys/picofarm/issues) and the bug-report form. Include version/commit, installation method, protocol/model when relevant, exact steps, expected/actual behavior and sanitized logs. Search existing reports first. Never attach secrets, private network details or an unredacted database. For vulnerabilities use [private reporting](../SECURITY.md).

## Future installation work

- [ ] Provide and validate an end-user installation without Docker: distributable binary/frontend, service lifecycle, persistent paths, upgrades and recovery. Source development commands are not a substitute for that installation workflow.
