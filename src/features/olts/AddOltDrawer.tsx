import { useState } from "react";
import { DetailDrawer } from "../../components/common/DetailDrawer";

interface AddOltDrawerProps {
  onClose: () => void;
}

interface FormState {
  name: string;
  vendor: string;
  model: string;
  technology: string;
  address: string;
  port: string;
  connection: string;
  credentialRef: string;
  profile: string;
}

const INITIAL: FormState = {
  name: "",
  vendor: "VSOL",
  model: "",
  technology: "GPON",
  address: "",
  port: "22",
  connection: "ssh",
  credentialRef: "",
  profile: "standard-5m",
};

const IPV4 = /^(\d{1,3}\.){3}\d{1,3}$/;

/**
 * Validation is pure and runs on every render, so the submit button reflects
 * the form's real state instead of a flag set by a click handler.
 */
function validate(form: FormState): Record<string, string> {
  const errors: Record<string, string> = {};

  if (form.name.trim() === "") errors.name = "Display name is required";
  if (form.model.trim() === "") errors.model = "Model is required";
  if (!IPV4.test(form.address)) errors.address = "IPv4 address expected";
  const port = Number(form.port);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    errors.port = "Port must be 1–65535";
  }
  if (form.credentialRef.trim() === "") {
    errors.credentialRef = "Reference a stored credential — never type the password here";
  }
  return errors;
}

export function AddOltDrawer({ onClose }: AddOltDrawerProps) {
  const [form, setForm] = useState<FormState>(INITIAL);
  const [attempted, setAttempted] = useState(false);
  const errors = validate(form);
  const isValid = Object.keys(errors).length === 0;

  const field = (key: keyof FormState) => ({
    value: form[key],
    onChange: (event: { target: { value: string } }) =>
      setForm((current) => ({ ...current, [key]: event.target.value })),
  });

  return (
    <DetailDrawer
      title="Add OLT"
      subtitle="Inventory record only — nothing is stored until the API exists"
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="btn btn--primary"
            disabled={!isValid}
            onClick={() => setAttempted(true)}
          >
            Validate
          </button>
        </>
      }
    >
      <form className="form" onSubmit={(event) => event.preventDefault()}>
        <label className="form__field">
          <span className="form__label">Display name</span>
          <input className="form__control" {...field("name")} placeholder="GPON-OLT-04" />
          {attempted && errors.name ? <span className="form__error">{errors.name}</span> : null}
        </label>

        <div className="form__row">
          <label className="form__field">
            <span className="form__label">Vendor</span>
            <select className="form__control" {...field("vendor")}>
              <option value="VSOL">VSOL</option>
              <option value="C-Data">C-Data</option>
            </select>
          </label>
          <label className="form__field">
            <span className="form__label">Model</span>
            <input className="form__control" {...field("model")} placeholder="GPON-OLT" />
            {attempted && errors.model ? <span className="form__error">{errors.model}</span> : null}
          </label>
        </div>

        <div className="form__row">
          <label className="form__field">
            <span className="form__label">Technology</span>
            <select className="form__control" {...field("technology")}>
              <option value="GPON">GPON</option>
              <option value="EPON">EPON</option>
            </select>
          </label>
          <label className="form__field">
            <span className="form__label">Connection</span>
            <select className="form__control" {...field("connection")}>
              <option value="ssh">SSH (preferred)</option>
              <option value="telnet">Telnet (management network only)</option>
            </select>
          </label>
        </div>

        <div className="form__row">
          <label className="form__field">
            <span className="form__label">Management address</span>
            <input className="form__control mono" {...field("address")} placeholder="192.0.2.20" />
            {attempted && errors.address ? <span className="form__error">{errors.address}</span> : null}
          </label>
          <label className="form__field form__field--narrow">
            <span className="form__label">Port</span>
            <input className="form__control mono" {...field("port")} inputMode="numeric" />
            {attempted && errors.port ? <span className="form__error">{errors.port}</span> : null}
          </label>
        </div>

        <label className="form__field">
          <span className="form__label">Credential reference</span>
          <input
            className="form__control mono"
            {...field("credentialRef")}
            placeholder="vault/olt/gpon-04"
          />
          {attempted && errors.credentialRef ? (
            <span className="form__error">{errors.credentialRef}</span>
          ) : null}
          <span className="form__hint">
            A reference to a stored secret. This form never accepts a password, and the
            browser never keeps one.
          </span>
        </label>

        <label className="form__field">
          <span className="form__label">Collection profile</span>
          <select className="form__control" {...field("profile")}>
            <option value="standard-5m">standard-5m</option>
            <option value="slow-15m">slow-15m</option>
            <option value="manual">manual</option>
          </select>
        </label>

        {form.connection === "telnet" ? (
          <p className="form__warning">
            Telnet carries credentials in clear text. It is only permitted on the isolated
            management network.
          </p>
        ) : null}

        {attempted ? (
          <p className="form__notice" role="status">
            Validated locally. Not saved — the write API arrives in stage 7, so no device
            record was created.
          </p>
        ) : null}
      </form>
    </DetailDrawer>
  );
}
