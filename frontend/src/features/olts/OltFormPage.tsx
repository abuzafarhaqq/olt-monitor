import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { PageHeader } from "../../components/common/PageHeader";
import { PermissionGate } from "../../components/common/PermissionGate";
import { LoadingState } from "../../components/common/States";
import { useResource } from "../../hooks/useResource";
import { dataSource } from "../../services/data";
import {
  WRITES_AVAILABLE,
  createOlt,
  deleteOlt,
  getCredentialMeta,
  setCredential,
  updateOlt,
  type CredentialMeta,
  type OltInput,
} from "../../services/oltApi";
import type { Olt } from "../../types/domain";
import { formatDateTime } from "../../utils/format";

interface FormState extends OltInput {
  username: string;
  password: string;
}

const EMPTY: FormState = {
  name: "",
  vendor: "VSOL",
  model: "",
  technology: "GPON",
  managementAddress: "",
  managementPort: 22,
  connection: "ssh",
  enabled: true,
  username: "",
  password: "",
};

const IPV4 = /^(\d{1,3}\.){3}\d{1,3}$/;

function validate(form: FormState, isEdit: boolean): Record<string, string> {
  const errors: Record<string, string> = {};

  if (form.name.trim() === "") errors.name = "Display name is required";
  if (form.vendor.trim() === "") errors.vendor = "Vendor is required";
  if (form.model.trim() === "") errors.model = "Model is required";
  if (!IPV4.test(form.managementAddress)) errors.managementAddress = "IPv4 address expected";
  if (!Number.isInteger(form.managementPort) || form.managementPort < 1 || form.managementPort > 65535) {
    errors.managementPort = "Port must be 1–65535";
  }

  // A new device needs a username. The password may be left blank only when a
  // credential already exists (edit) — otherwise there would be nothing to dial.
  if (form.username.trim() === "" && !isEdit) errors.username = "Username is required";
  if (!isEdit && form.password === "") errors.password = "Password is required";
  if (form.connection === "telnet" && form.password !== "" && form.password.length < 8) {
    errors.password = "Telnet sends this in clear text — use at least 8 characters";
  }

  return errors;
}

export function OltFormPage() {
  const { oltId } = useParams<{ oltId: string }>();
  const navigate = useNavigate();
  const isEdit = oltId !== undefined;

  const { data: existing, loading: loadingDevice } = useResource<Olt | null>(
    () => (isEdit && oltId !== undefined ? dataSource.olt(oltId) : Promise.resolve(null)),
    [isEdit, oltId],
  );

  const [form, setForm] = useState<FormState>(EMPTY);

  // The form starts empty and is filled when the device arrives. Prefilling from
  // a fixture instead would show stale values the moment the API disagrees.
  useEffect(() => {
    if (existing === null) return;
    setForm({
      ...EMPTY,
      name: existing.name,
      vendor: existing.vendor,
      model: existing.model,
      technology: existing.technology,
      managementAddress: existing.managementAddress,
      connection: existing.connection,
      enabled: true,
    });
  }, [existing]);
  const [meta, setMeta] = useState<CredentialMeta | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [status, setStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [problem, setProblem] = useState<string | null>(null);
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  useEffect(() => {
    if (!isEdit || oltId === undefined) return;
    let alive = true;
    getCredentialMeta(oltId).then((result) => {
      if (alive) setMeta(result);
    });
    return () => {
      alive = false;
    };
  }, [isEdit, oltId]);

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((current) => ({ ...current, [key]: value }));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setProblem(null);

    const found = validate(form, isEdit);
    setErrors(found);
    if (Object.keys(found).length > 0) return;

    setStatus("saving");
    try {
      const input: OltInput = {
        name: form.name.trim(),
        vendor: form.vendor.trim(),
        model: form.model.trim(),
        technology: form.technology,
        managementAddress: form.managementAddress.trim(),
        managementPort: form.managementPort,
        connection: form.connection,
        enabled: form.enabled,
      };

      const saved = isEdit ? await updateOlt(oltId, input) : await createOlt(input);

      if (form.password !== "") {
        await setCredential(saved.id, form.username.trim(), form.password);
        // The password must not survive the request.
        setForm((current) => ({ ...current, password: "" }));
      }

      setStatus("saved");
      navigate(`/olts/${saved.id}`);
    } catch (caught) {
      setStatus("idle");
      setProblem(caught instanceof Error ? caught.message : "Could not save the device");
    }
  };

  if (isEdit && loadingDevice) {
    return <LoadingState label="Loading device…" />;
  }

  if (isEdit && existing === null) {
    return (
      <>
        <PageHeader title="OLT not found" subtitle={`No device with id ${oltId ?? "(none)"}`} />
        <p>
          <Link className="btn btn--small" to="/olts">
            ← OLT Inventory
          </Link>
        </p>
      </>
    );
  }

  const title = isEdit ? `Edit ${existing?.name ?? "OLT"}` : "Add OLT";

  return (
    <>
      <PageHeader
        title={title}
        subtitle={
          isEdit
            ? "Changing the address or credential affects the next collection, not the last one."
            : "The device is created without state — it looks unknown until the first collection."
        }
        actions={
          <Link className="btn btn--small" to="/olts">
            ← OLT Inventory
          </Link>
        }
      />

      {!WRITES_AVAILABLE ? (
        <div className="callout" role="note">
          <strong>Mock build.</strong> This form validates locally, but saving needs the Go API
          (<code className="mono">USE_MOCK_API = false</code> in{" "}
          <code className="mono">services/client.ts</code>).
        </div>
      ) : null}

      <PermissionGate
        permission="device.manage"
        fallback={
          <div className="callout" role="note">
            <strong>Read-only for your role.</strong> Device management needs the Network Engineer or
            Administrator role.
          </div>
        }
      >
        <form className="form form--page" onSubmit={submit}>
          <section className="card" aria-label="Device">
            <div className="card__header">
              <h2 className="card__title">Device</h2>
              <span className="card__aside">friendly name and vendor identification</span>
            </div>
            <div className="card__body form__grid">
              <label className="form__field">
                <span className="form__label">Display name</span>
                <input
                  className="form__control"
                  value={form.name}
                  onChange={(event) => update("name", event.target.value)}
                  placeholder="EPON-OLT-01"
                />
                {errors.name ? <span className="form__error">{errors.name}</span> : null}
              </label>

              <label className="form__field">
                <span className="form__label">Vendor</span>
                <select
                  className="form__control"
                  value={form.vendor}
                  onChange={(event) => update("vendor", event.target.value)}
                >
                  <option value="VSOL">VSOL</option>
                  <option value="C-Data">C-Data</option>
                </select>
              </label>

              <label className="form__field">
                <span className="form__label">Model</span>
                <input
                  className="form__control"
                  value={form.model}
                  onChange={(event) => update("model", event.target.value)}
                  placeholder="EPON-OLT"
                />
                {errors.model ? <span className="form__error">{errors.model}</span> : null}
              </label>

              <label className="form__field">
                <span className="form__label">Technology</span>
                <select
                  className="form__control"
                  value={form.technology}
                  onChange={(event) => update("technology", event.target.value as "EPON" | "GPON")}
                >
                  <option value="GPON">GPON</option>
                  <option value="EPON">EPON</option>
                </select>
                <span className="form__hint">Thresholds are looked up by vendor + technology.</span>
              </label>
            </div>
          </section>

          <section className="card" aria-label="Management access">
            <div className="card__header">
              <h2 className="card__title">Management access</h2>
              <span className="card__aside">reachable only from the management network</span>
            </div>
            <div className="card__body form__grid">
              <label className="form__field">
                <span className="form__label">Management address</span>
                <input
                  className="form__control mono"
                  value={form.managementAddress}
                  onChange={(event) => update("managementAddress", event.target.value)}
                  placeholder="192.0.2.20"
                />
                {errors.managementAddress ? (
                  <span className="form__error">{errors.managementAddress}</span>
                ) : null}
              </label>

              <label className="form__field form__field--narrow">
                <span className="form__label">Port</span>
                <input
                  className="form__control mono"
                  inputMode="numeric"
                  value={form.managementPort}
                  onChange={(event) => update("managementPort", Number(event.target.value))}
                />
                {errors.managementPort ? (
                  <span className="form__error">{errors.managementPort}</span>
                ) : null}
              </label>

              <label className="form__field">
                <span className="form__label">Transport</span>
                <select
                  className="form__control"
                  value={form.connection}
                  onChange={(event) => update("connection", event.target.value as "ssh" | "telnet")}
                >
                  <option value="ssh">SSH (preferred)</option>
                  <option value="telnet">Telnet (management network only)</option>
                </select>
              </label>

              <label className="checkbox form__field">
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={(event) => update("enabled", event.target.checked)}
                />
                Enabled for collection
              </label>
            </div>
          </section>

          <section className="card" aria-label="Credentials">
            <div className="card__header">
              <h2 className="card__title">Credentials</h2>
              <span className="card__aside">
                {meta === null
                  ? "no credential stored"
                  : `set ${formatDateTime(meta.updatedAt)} by ${meta.updatedBy ?? "unknown"}`}
              </span>
            </div>
            <div className="card__body form__grid">
              <label className="form__field">
                <span className="form__label">Username</span>
                <input
                  className="form__control"
                  autoComplete="off"
                  value={form.username}
                  onChange={(event) => update("username", event.target.value)}
                  placeholder={meta?.username ?? "admin"}
                />
                {errors.username ? <span className="form__error">{errors.username}</span> : null}
              </label>

              <label className="form__field">
                <span className="form__label">
                  {isEdit && meta !== null ? "New password (leave blank to keep)" : "Password"}
                </span>
                <input
                  className="form__control"
                  type="password"
                  autoComplete="new-password"
                  value={form.password}
                  onChange={(event) => update("password", event.target.value)}
                />
                {errors.password ? <span className="form__error">{errors.password}</span> : null}
              </label>

              <p className="form__hint form__grid-full">
                The password is encrypted on the server and can never be read back — not by this
                page, not by an administrator. To change it, set a new one.
              </p>

              {form.connection === "telnet" ? (
                <p className="form__warning form__grid-full">
                  Telnet carries the credential in clear text. Only permitted on the isolated
                  management network.
                </p>
              ) : null}
            </div>
          </section>

          {problem === null ? null : (
            <p className="form__error" role="alert">
              {problem}
            </p>
          )}

          {isEdit ? (
            <section className="card card--danger" aria-label="Delete device">
              <div className="card__header">
                <h2 className="card__title">Delete device</h2>
                <span className="card__aside">removes its ports, ONUs, events and incidents</span>
              </div>
              <div className="card__body">
                {confirmingDelete ? (
                  <div className="form__actions">
                    <button type="button" className="btn" onClick={() => setConfirmingDelete(false)}>
                      Cancel
                    </button>
                    <button
                      type="button"
                      className="btn btn--danger"
                      disabled={status === "saving"}
                      onClick={async () => {
                        setStatus("saving");
                        try {
                          await deleteOlt(oltId);
                          navigate("/olts");
                        } catch (caught) {
                          setStatus("idle");
                          setProblem(caught instanceof Error ? caught.message : "Could not delete");
                        }
                      }}
                    >
                      Delete {existing?.name} permanently
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    className="btn"
                    onClick={() => setConfirmingDelete(true)}
                  >
                    Delete this device…
                  </button>
                )}
              </div>
            </section>
          ) : null}

          <div className="form__actions">
            <Link className="btn" to="/olts">
              Cancel
            </Link>
            <button type="submit" className="btn btn--primary" disabled={status === "saving"}>
              {status === "saving" ? "Saving…" : isEdit ? "Save changes" : "Create device"}
            </button>
          </div>
        </form>
      </PermissionGate>

      <p className="page-footnote">
        “Test connection” is deliberately absent: dialling a production OLT runs a command on it,
        and that happens only with explicit approval.
      </p>
    </>
  );
}
