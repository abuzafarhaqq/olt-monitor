import { useAuth } from "../../app/AuthContext";
import { PageHeader } from "../../components/common/PageHeader";
import { ROLES, ROLE_PERMISSIONS, can, type Permission } from "../../services/auth";

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

export function UsersAccessPage() {
  const { user } = useAuth();

  return (
    <>
      <PageHeader
        title="Users & Access"
        subtitle="Role definitions and what each one may do. The API enforces these; this page only describes them."
        isMock
      />

      <section className="card" aria-label="Current user">
        <div className="card__body">
          <div className="drawer-section__row">
            <span className="badge badge--info">signed in as</span>
            <strong>{user?.fullName ?? "not signed in"}</strong>
            <span className="badge badge--investigating">
              {ROLES.find((role) => role.id === user?.role)?.label ?? "—"}
            </span>
            <span className="cell-sub">
              Sign in as a different account to see what each role hides.
            </span>
          </div>
        </div>
      </section>

      <section className="card" aria-label="Roles" style={{ marginTop: "var(--space-4)" }}>
        <div className="card__header">
          <h2 className="card__title">Roles</h2>
          <span className="card__aside">{ROLES.length} defined</span>
        </div>
        <div className="card__body">
          <ul className="role-list">
            {ROLES.map((role) => (
              <li className="role-row" key={role.id}>
                <div>
                  <div className="role-row__name">{role.label}</div>
                  <div className="cell-sub">{role.description}</div>
                </div>
                <span className="badge badge--unknown">
                  {ROLE_PERMISSIONS[role.id].length} permissions
                </span>
              </li>
            ))}
          </ul>
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

      <p className="page-footnote">
        Hiding a button is not authorisation. Every endpoint re-checks the caller's role; the
        matrix above exists so the two cannot drift apart unnoticed.
      </p>
    </>
  );
}
