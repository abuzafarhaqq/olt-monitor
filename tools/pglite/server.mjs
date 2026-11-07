// A real PostgreSQL, compiled to WebAssembly, exposed over the wire protocol.
//
// Why this exists: the sandbox this backend is developed in cannot start a
// native server (seccomp denies shmget, no /dev/shm), but the SQL still has to
// be executed by something before anyone trusts it. PGlite runs the actual
// Postgres core in userspace, so the migration, the seed and every query get
// parsed and executed by Postgres — not by a mock.
//
// Node 18 has no global CustomEvent; pglite-socket dispatches one on listen.
if (typeof globalThis.CustomEvent === "undefined") {
  globalThis.CustomEvent = class CustomEvent extends Event {
    constructor(type, init = {}) {
      super(type);
      this.detail = init.detail;
    }
  };
}

const { PGlite } = await import("@electric-sql/pglite");
const { PGLiteSocketServer } = await import("@electric-sql/pglite-socket");

const port = Number(process.env.PGLITE_PORT ?? 5433);
const db = await PGlite.create();
const server = new PGLiteSocketServer({ db, port, host: "127.0.0.1", maxConnections: 5 });

await server.start();
console.log(`pglite listening on 127.0.0.1:${port}`);

const shutdown = async () => {
  await server.stop();
  await db.close();
  process.exit(0);
};
process.on("SIGTERM", shutdown);
process.on("SIGINT", shutdown);
