export const metadata = {
  title: "Deployment - PicoFarm Docs",
  description:
    "Self-host PicoFarm as a web app: build from source or run the Docker image, with the real environment variables.",
};

export default function DeploymentDocs() {
  return (
    <div className="docs-prose">
      <h1>Deployment</h1>
      <p className="text-lg !text-surface-300">
        PicoFarm is a browser application. Run it on your own server or NAS with
        Docker, or build the binary and serve it directly. Your data, your
        infrastructure.
      </p>

      <h2>Overview</h2>
      <p>
        There is no desktop shell. A single binary serves both the JSON API and
        the built frontend from <code>web/dist</code> on the same port, and the
        database is an embedded SQLite file. The only thing you need to persist
        is that file plus your uploads directory.
      </p>

      <h2>Docker</h2>
      <p>
        The repository ships a multi-stage <code>Dockerfile</code> that builds
        the frontend, compiles the Go server, and publishes a small runtime
        image. It listens on port <code>8084</code> and stores everything under
        <code> /data</code>.
      </p>

      <pre><code>{`# Build the image
docker build -t picofarm .

# Run with a named volume for data persistence
docker run -d \\
  --name picofarm \\
  -p 8084:8084 \\
  -v picofarm-data:/data \\
  picofarm`}</code></pre>

      <p>
        The image sets <code>PORT=8084</code>,{" "}
        <code>DATABASE_PATH=/data/picofarm.db</code> and{" "}
        <code>UPLOAD_DIR=/data/uploads</code> for you, and declares{" "}
        <code>/data</code> as a volume, so mounting one path is enough. It runs
        as a non-root user and ships a health check against{" "}
        <code>/health</code>.
      </p>

      <h3>Docker features</h3>
      <ul>
        <li>Multi-stage build for a minimal runtime image</li>
        <li>Embedded SQLite database (no external database required)</li>
        <li>Single volume for the database and uploads</li>
        <li>Health check endpoint at <code>/health</code></li>
      </ul>

      <h2>Build from source</h2>
      <pre><code>{`# Clone the repository
git clone https://github.com/Brook-sys/picofarm.git
cd picofarm

# Install dependencies
make deps

# Production build: backend binary + frontend assets
make build

# Serve on :8084
./bin/server`}</code></pre>

      <p>
        For frontend development against a live backend, run{" "}
        <code>make dev</code>: the Vite server comes up on{" "}
        <code>http://localhost:5173</code> and proxies <code>/api</code> to the
        Go server on port <code>8084</code>.
      </p>

      <h2>Environment variables</h2>
      <p>
        These are the variables the code actually reads. Anything else you may
        find in older guides does nothing.
      </p>
      <table>
        <thead>
          <tr>
            <th>Variable</th>
            <th>Default</th>
            <th>Description</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td><code>PORT</code></td>
            <td><code>8084</code></td>
            <td>HTTP port for the API and the served frontend</td>
          </tr>
          <tr>
            <td><code>DATABASE_PATH</code></td>
            <td><code>~/.picofarm/picofarm.db</code></td>
            <td>SQLite database file path</td>
          </tr>
          <tr>
            <td><code>UPLOAD_DIR</code></td>
            <td><code>./uploads</code></td>
            <td>Directory for uploads and generated files</td>
          </tr>
          <tr>
            <td><code>STATIC_DIR</code></td>
            <td><code>./web/dist</code></td>
            <td>Built frontend assets served by the backend</td>
          </tr>
          <tr>
            <td><code>ALLOWED_ORIGINS</code></td>
            <td>unset</td>
            <td>
              Comma-separated CORS origins. Unset allows local development
              origins only
            </td>
          </tr>
          <tr>
            <td><code>API_TOKEN</code></td>
            <td>unset</td>
            <td>
              When set, every request needs{" "}
              <code>Authorization: Bearer &lt;token&gt;</code>. Note that this
              also blocks the Etsy webhook unless the caller presents the token
            </td>
          </tr>
          <tr>
            <td><code>ENCRYPTION_KEY</code></td>
            <td>unset</td>
            <td>Key used to encrypt integration settings at rest</td>
          </tr>
          <tr>
            <td><code>ENVIRONMENT</code></td>
            <td>unset</td>
            <td>Set to <code>production</code> for production behavior</td>
          </tr>
          <tr>
            <td><code>SENTRY_DSN</code></td>
            <td>unset</td>
            <td>Optional error reporting</td>
          </tr>
          <tr>
            <td><code>ETSY_CLIENT_ID</code></td>
            <td>&mdash;</td>
            <td>Etsy OAuth application ID</td>
          </tr>
          <tr>
            <td><code>ETSY_REDIRECT_URI</code></td>
            <td>
              <code>http://localhost:8084/api/integrations/etsy/callback</code>
            </td>
            <td>Etsy OAuth callback URL; point it at your public host</td>
          </tr>
          <tr>
            <td><code>ETSY_WEBHOOK_SECRET</code></td>
            <td>&mdash;</td>
            <td>
              When set, webhook events with a missing or invalid signature are
              rejected with <code>401</code>
            </td>
          </tr>
        </tbody>
      </table>

      <div className="docs-callout">
        <strong>Tip:</strong> LAN-connected printers are discovered from the
        server, so Docker on the same network as your printers works fine —
        there is no desktop app to bridge anything. If you expose the instance
        beyond your LAN, set <code>ALLOWED_ORIGINS</code> and{" "}
        <code>API_TOKEN</code>.
      </div>
    </div>
  );
}
