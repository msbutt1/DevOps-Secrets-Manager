import { Link } from 'react-router-dom';
import { Panel } from '@/components/win95';
import { Shield, AlertTriangle } from 'lucide-react';
import { useDocumentTitle } from '@/hooks/use-document-title';

const LAST_UPDATED = '16 September 2026';
const CONTACT = 'msbutt112004@gmail.com';

const Section = ({
  id,
  title,
  children,
}: {
  id: string;
  title: string;
  children: React.ReactNode;
}) => (
  <section aria-labelledby={id} className="mb-6">
    <h2 id={id} className="font-bold text-base mb-2">
      {title}
    </h2>
    <div className="space-y-2 text-sm leading-relaxed">{children}</div>
  </section>
);

/**
 * Privacy and terms for the hosted instance. Written to be read rather than clicked past: it
 * says what is stored, what is not encrypted, and that whoever operates the instance holds the
 * master key and can therefore decrypt anything kept here.
 */
export const LegalPage = () => {
  useDocumentTitle('Privacy and terms');

  return (
    <div className="min-h-screen bg-[#008080] p-4 flex justify-center">
      <div className="win-border-raised bg-background w-full max-w-[760px] my-4 h-fit">
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Shield size={14} strokeWidth={1.5} />
            <span>Vault Console — Privacy and Terms</span>
          </div>
        </div>

        <div className="p-win-md">
          <h1 className="font-bold text-lg mb-1">Privacy and terms</h1>
          <p className="text-sm mb-4">
            Last updated {LAST_UPDATED}. This describes how this particular deployment handles your
            data. It is a plain description of what the software does, not legal advice.
          </p>

          <Panel className="mb-6">
            <div className="flex items-start gap-3">
              <AlertTriangle size={24} className="text-warning flex-shrink-0" strokeWidth={1.5} />
              <p className="text-sm">
                <strong>Read this part if you read nothing else.</strong> Secret values are
                encrypted, but the person who runs this service holds the master key and can decrypt
                anything stored here. Names, descriptions and labels are not encrypted at all. This
                is a personal project on free hosting, with no backup guarantee and no multi-factor
                authentication. Do not store anything whose loss or disclosure would genuinely hurt
                you.
              </p>
            </div>
          </Panel>

          <Section id="who" title="Who runs this">
            <p>
              This instance is operated by an individual as a personal project, not by a company.
              There is no support team and no service agreement. Questions, deletion requests and
              security reports go to <strong>{CONTACT}</strong>.
            </p>
          </Section>

          <Section id="stored" title="What is stored">
            <ul className="list-disc pl-5 space-y-1">
              <li>
                <strong>Your account:</strong> email address, the display name you choose, and a
                bcrypt hash of your password. The password itself is never stored.
              </li>
              <li>
                <strong>Secret values:</strong> encrypted with AES-256-GCM using a key unique to
                each vault, which is itself encrypted with a master key kept in the hosting
                platform&apos;s secret store rather than in the database.
              </li>
              <li>
                <strong>Not encrypted:</strong> secret names, descriptions and labels, and vault,
                environment and organization names. These are stored as ordinary text, so choose
                names that are not themselves sensitive.
              </li>
              <li>
                <strong>Activity:</strong> an audit record of actions such as reveals, exports,
                logins and membership changes, each with your user account, the time, your IP
                address and your browser&apos;s user agent. Your signed-in sessions store the same,
                so you can recognise and revoke them.
              </li>
            </ul>
          </Section>

          <Section id="who-can-read" title="Who can read your secrets">
            <p>
              You, and anyone you give access to a vault. Roles limit who can reveal values, and
              every reveal and export is recorded in the audit log.
            </p>
            <p>
              <strong>The operator can also read them.</strong> Decryption needs the master key and
              the database, and the operator has both. Nothing in the design prevents this, and no
              policy makes it untrue, so it is stated here rather than implied.
            </p>
          </Section>

          <Section id="third-parties" title="Services this depends on">
            <p>
              Running this involves other companies, which necessarily see some of your data in
              transit or at rest: <strong>Render</strong> runs the application,{' '}
              <strong>Neon</strong> hosts the database, <strong>Cloudflare</strong> serves the site
              and carries the traffic, and <strong>Resend</strong> delivers verification, invitation
              and password reset email, so it sees the recipient address. Each has its own terms and
              its own jurisdiction.
            </p>
          </Section>

          <Section id="cookies" title="Cookies and tracking">
            <p>
              One cookie is set, holding your session so that a page reload does not sign you out.
              It is <code>HttpOnly</code> and cannot be read by scripts. There is no analytics, no
              advertising and no third-party tracking of any kind, and nothing is shared with anyone
              for those purposes.
            </p>
          </Section>

          <Section id="retention" title="Keeping and deleting your data">
            <p>
              Data is kept until you delete it. Deleting a secret removes its stored value and its
              history; the audit record that it once existed remains, because an audit log that can
              be edited is not an audit log.
            </p>
            <p>
              <strong>There is no self-service account deletion yet.</strong> Email {CONTACT} and
              the account and its vaults will be deleted. Database backups are kept for a limited
              period, so a copy may survive briefly in a backup before it ages out.
            </p>
          </Section>

          <Section id="limits" title="What this does not provide">
            <ul className="list-disc pl-5 space-y-1">
              <li>No multi-factor authentication and no single sign-on.</li>
              <li>No uptime guarantee. It runs on free hosting and may be slow, down or gone.</li>
              <li>
                No guarantee your data survives. Keep your own copy of anything you cannot lose.
              </li>
              <li>
                No warranty of any kind. The software is provided as-is, and the operator is not
                liable for loss or damage arising from using it.
              </li>
            </ul>
          </Section>

          <Section id="use" title="Using it fairly">
            <p>
              Do not use this service to break the law, to store material you have no right to, or
              to attack the service or its other users. Accounts doing so will be removed. The
              operator may change or shut down the service, with as much notice as circumstances
              allow.
            </p>
            <p>
              If you find a security problem, please report it privately to {CONTACT} rather than
              publicly, and it will be fixed and credited.
            </p>
          </Section>

          <div className="mt-6 pt-4 border-t border-border text-sm">
            <Link to="/login" className="underline">
              Back to sign in
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
};

export default LegalPage;
