import { useEffect, useState, type FormEvent } from "react";
import { useAuth } from "../../app/AuthContext";
import { DataTable, type Column } from "../../components/common/DataTable";
import { DetailDrawer } from "../../components/common/DetailDrawer";
import { MetricCard } from "../../components/common/MetricCard";
import { PageHeader } from "../../components/common/PageHeader";
import { PermissionGate } from "../../components/common/PermissionGate";
import { LoadingState } from "../../components/common/States";
import { ROLES, can, type Permission, type Role } from "../../services/auth";
import {
  USER_WRITES_AVAILABLE,
  createUser,
  deleteUser,
  listUsers,
  resetPassword,
  updateUser,
  type UserRecord,
} from "../../services/userApi";
import { formatDateTime } from "../../utils/format";

const PERMISSIONS: { id: Permission; label: string }[] = [
  { id: "device.read", label: "View devices" },
  { id: "device.manage", label: "Manage devices" },
  { id: "diagnostics.read", label: "View diagnostics" },
  { id: "diagnostics.run", label: "Run diagnostic commands" },
  { id: "incident.read", label: "View incidents" },
  { id: "incident.acknowledge", label: "Acknowledge incidents" },
  { id: "incident.note", label: "Add incident notes" },
  { id: "incident.resolve", label: "Resolve incidents" },
  { id: "reports.read", label: "View reports" },
  { id: "reports.export", label: "Export reports" },
  { id: "settings.read", label: "View settings" },
  { id: "settings.manage", label: "Change settings" },
  { id: "users.manage", label: "Manage users and roles" },
];

type DraftMode = { kind: "create" } | { kind: "edit"; user: UserRecord } | { kind: "password"; user: UserRecord };

interface Draft {
  username: string;
  fullName: string;
  email: string;
  phone: string;
  role: Role;
  password: string;
  disabled: boolean;
}

const EMPTY_DRAFT: Draft = {
  username: "",
  fullName: "",
  email: "",
  phone: "",
  role: "viewer",
  password: "",
  disabled: false,
};

export function UsersAccessPage() {
  const { user: me, can: canI } = useAuth();
  const [users, setUsers] = useState<UserRecord[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<DraftMode | null>(null);
  const [draft, setDraft] = useState<Draft>(EMPTY_DRAFT);
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  const reload = () => {
    setError(null);
    listUsers()
      .then(setUsers)
      .catch((caught: unknown) =>
        setError(caught instanceof Error ? caught.message : "Could not load accounts"),
      );
  };

  useEffect(reload, []);

  const open = (next: DraftMode) => {
    setMode(next);
    setError(null);
    setDraft(
      next.kind === "create"
        ? EMPTY_DRAFT
        : {
            username: next.user.username,
            fullName: next.user.fullName,
            email: next.user.email,
            phone: next.user.phone ?? "",
            role: next.user.role,
            password: "",
            disabled: next.user.disabled,
          },
    );
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (mode === null) return;
    setBusy(true);
    setError(null);
    try {
      if (mode.kind === "create") {
        await createUser({
          username: draft.username,
          fullName: draft.fullName,
          email: draft.email,
          phone: draft.phone,
          role: draft.role,
          password: draft.password,
        });
      } else if (mode.kind === "edit") {
        await updateUser(mode.user.id, {
          fullName: draft.fullName,
          email: draft.email,
          phone: draft.phone,
          role: draft.role,
          disabled: draft.disabled,
        });
      } else {
        await resetPassword(mode.user.id, draft.password);
      }
      setMode(null);
      setDraft(EMPTY_DRAFT);
      reload();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not save the account");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (record: UserRecord) => {
    setBusy(true);
    setError(null);
    try {
      await deleteUser(record.id);
      setConfirmDelete(null);
      reload();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not delete the account");
    } finally {
      setBusy(false);
    }
  };

  const columns: Column<UserRecord>[] = [
    {
      key: "name",
      header: "Account",
      render: (record) => (
        <div>
          <div className="data-table__cell-strong">{record.fullName}</div>
          <div className="cell-sub mono">{record.username}</div>
        </div>
      ),
    },
    { key: "email", header: "Email", render: (record) => record.email },
    {
      key: "role",
      header: "Role",
      render: (record) => (
        <span className="badge badge--investigating">
          {ROLES.find((role) => role.id === record.role)?.label ?? record.role}
        </span>
      ),
    },
    {
      key: "state",
      header: "State",
      render: (record) =>
        record.disabled ? (
          <span className="badge badge--unknown">disabled</span>
        ) : (
          <span className="badge badge--online">active</span>
        ),
    },
    {
      key: "lastLogin",
      header: "Last sign-in",
      render: (record) =>
        record.lastLoginAt === null ? (
          <span className="cell-sub">never</span>
        ) : (
          <span className="tabular">{formatDateTime(record.lastLoginAt)}</span>
        ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (record) => {
        if (!canI("users.manage")) return <span className="cell-sub">read-only</span>;
        if (confirmDelete === record.id) {
          return (
            <div className="row-actions">
              <button type="button" className="btn btn--small" onClick={() => setConfirmDelete(null)}>
                Cancel
              </button>
              <button
                type="button"
                className="btn btn--small btn--danger"
                disabled={busy}
                onClick={() => remove(record)}
              >
                Delete {record.username}
              </button>
            </div>
          );
        }
        return (
          <div className="row-actions">
            <button type="button" className="btn btn--small" onClick={() => open({ kind: "edit", user: record })}>
              Edit
            </button>
            <button
              type="button"
              className="btn btn--small"
              onClick={() => open({ kind: "password", user: record })}
            >
              Password
            </button>
            <button
              type="button"
              className="btn btn--small"
              onClick={() => setConfirmDelete(record.id)}
              disabled={record.id === me?.id}
              title={record.id === me?.id ? "an account cannot delete itself" : undefined}
            >
              Delete
            </button>
          </div>
        );
      },
    },
  ];

  return (
    <>
      <PageHeader
        title="Users & Access"
        subtitle="Accounts, roles and what each role may do. The API enforces these; this page only describes them."
        isMock={!USER_WRITES_AVAILABLE}
        actions={
          <PermissionGate permission="users.manage">
            <button type="button" className="btn btn--primary" onClick={() => open({ kind: "create" })}>
              Add user
            </button>
          </PermissionGate>
        }
      />

      {!USER_WRITES_AVAILABLE ? (
        <div className="callout" role="note">
          <strong>Mock build.</strong> These are the demo accounts, read-only. Account changes need
          the API.
        </div>
      ) : null}

      {error === null ? null : (
        <p className="form__error" role="alert">
          {error}
        </p>
      )}

      <section className="kpi-grid" aria-label="Account summary">
        <MetricCard label="Accounts" value={users?.length ?? "—"} />
        <MetricCard
          label="Active"
          value={users?.filter((record) => !record.disabled).length ?? "—"}
          tone="online"
        />
        <MetricCard
          label="Administrators"
          value={users?.filter((record) => record.role === "administrator" && !record.disabled).length ?? "—"}
          tone="investigating"
        />
        <MetricCard
          label="Disabled"
          value={users?.filter((record) => record.disabled).length ?? "—"}
          tone="unknown"
        />
      </section>

      <section className="card" aria-label="Accounts">
        <div className="card__header">
          <h2 className="card__title">Accounts</h2>
          <span className="card__aside">passwords are hashed — nobody can read them back</span>
        </div>
        <div className="card__body card__body--flush">
          {users === null ? (
            <LoadingState label="Loading accounts…" />
          ) : (
            <DataTable
              columns={columns}
              rows={users}
              getRowKey={(record) => record.id}
              caption="Accounts"
              emptyMessage="No account exists yet."
            />
          )}
        </div>
      </section>

      <section className="card" aria-label="Permission matrix" style={{ marginTop: "var(--space-4)" }}>
        <div className="card__header">
          <h2 className="card__title">Permission matrix</h2>
          <span className="card__aside">✓ = allowed</span>
        </div>
        <div className="card__body card__body--flush">
          <table className="data-table matrix">
            <thead>
              <tr>
                <th scope="col">Permission</th>
                {ROLES.map((role) => (
                  <th key={role.id} scope="col" className="align-right">
                    {role.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {PERMISSIONS.map((permission) => (
                <tr key={permission.id}>
                  <td>
                    <div>{permission.label}</div>
                    <div className="cell-sub mono">{permission.id}</div>
                  </td>
                  {ROLES.map((role) => (
                    <td key={role.id} className="align-right">
                      {can(role.id, permission.id) ? (
                        <span className="matrix__yes" aria-label="allowed">
                          ✓
                        </span>
                      ) : (
                        <span className="matrix__no" aria-label="denied">
                          ·
                        </span>
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {mode === null ? null : (
        <DetailDrawer
          title={
            mode.kind === "create"
              ? "Add user"
              : mode.kind === "edit"
                ? `Edit ${mode.user.username}`
                : `Reset password — ${mode.user.username}`
          }
          subtitle={
            mode.kind === "password"
              ? "Signs the account out on every device."
              : "Roles come from the policy table; the API re-checks every request."
          }
          onClose={() => setMode(null)}
          footer={
            <>
              <button type="button" className="btn" onClick={() => setMode(null)}>
                Cancel
              </button>
              <button
                type="submit"
                form="user-form"
                className="btn btn--primary"
                disabled={busy}
              >
                {busy ? "Saving…" : mode.kind === "create" ? "Create" : "Save"}
              </button>
            </>
          }
        >
          <form className="form" id="user-form" onSubmit={submit}>
            {mode.kind === "password" ? (
              <label className="form__field">
                <span className="form__label">New password</span>
                <input
                  className="form__control"
                  type="password"
                  autoComplete="new-password"
                  value={draft.password}
                  onChange={(event) => setDraft({ ...draft, password: event.target.value })}
                  minLength={8}
                  required
                />
              </label>
            ) : (
              <>
                {mode.kind === "create" ? (
                  <label className="form__field">
                    <span className="form__label">Username</span>
                    <input
                      className="form__control mono"
                      value={draft.username}
                      onChange={(event) => setDraft({ ...draft, username: event.target.value })}
                      required
                    />
                  </label>
                ) : null}

                <label className="form__field">
                  <span className="form__label">Full name</span>
                  <input
                    className="form__control"
                    value={draft.fullName}
                    onChange={(event) => setDraft({ ...draft, fullName: event.target.value })}
                    required
                  />
                </label>

                <label className="form__field">
                  <span className="form__label">Email</span>
                  <input
                    className="form__control"
                    type="email"
                    value={draft.email}
                    onChange={(event) => setDraft({ ...draft, email: event.target.value })}
                    required
                  />
                </label>

                <label className="form__field">
                  <span className="form__label">Phone (optional)</span>
                  <input
                    className="form__control"
                    value={draft.phone}
                    onChange={(event) => setDraft({ ...draft, phone: event.target.value })}
                  />
                </label>

                <label className="form__field">
                  <span className="form__label">Role</span>
                  <select
                    className="form__control"
                    value={draft.role}
                    onChange={(event) => setDraft({ ...draft, role: event.target.value as Role })}
                  >
                    {ROLES.map((role) => (
                      <option key={role.id} value={role.id}>
                        {role.label} — {role.description}
                      </option>
                    ))}
                  </select>
                </label>

                {mode.kind === "create" ? (
                  <label className="form__field">
                    <span className="form__label">Initial password</span>
                    <input
                      className="form__control"
                      type="password"
                      autoComplete="new-password"
                      value={draft.password}
                      onChange={(event) => setDraft({ ...draft, password: event.target.value })}
                      minLength={8}
                      required
                    />
                    <span className="form__hint">
                      At least 8 characters. The account holder should change it after signing in.
                    </span>
                  </label>
                ) : (
                  <label className="checkbox form__field">
                    <input
                      type="checkbox"
                      checked={draft.disabled}
                      onChange={(event) => setDraft({ ...draft, disabled: event.target.checked })}
                    />
                    Disabled (cannot sign in; sessions are not revoked until expiry)
                  </label>
                )}
              </>
            )}
          </form>
        </DetailDrawer>
      )}
    </>
  );
}
