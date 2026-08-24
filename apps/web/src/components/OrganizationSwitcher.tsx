import { useCurrentOrganization } from '@/contexts/OrganizationContext';

/** Title-bar selector for people who belong to more than one organization. */
export const OrganizationSwitcher = () => {
  const { organizations, currentOrganization, setCurrentOrganizationId } = useCurrentOrganization();

  if (organizations.length < 2 || !currentOrganization) return null;

  return (
    <div className="flex items-center gap-1">
      <label htmlFor="organization-switcher" className="text-win-small text-muted-foreground">
        Organization:
      </label>
      <select
        id="organization-switcher"
        className="win-input !py-0 text-win-small max-w-[220px]"
        value={currentOrganization.id}
        onChange={(e) => setCurrentOrganizationId(e.target.value)}
      >
        {organizations.map((org) => (
          <option key={org.id} value={org.id}>
            {org.name}
          </option>
        ))}
      </select>
    </div>
  );
};
