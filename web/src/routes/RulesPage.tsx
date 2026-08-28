import { useState } from 'react';
import {
  ApiError,
  type SiteRule,
  type SiteRuleNew,
  type SiteRulePatch,
} from '@nextchapter/api-client';
import {
  useCreateRule,
  useDeleteRule,
  usePatchRule,
  useSites,
} from '../lib/queries';
import { RuleForm } from '../components/RuleForm';
import { ConfirmInline } from '../components/ConfirmInline';
import { ErrorBanner } from '../components/ErrorBanner';

type FormState =
  | { mode: 'closed' }
  | { mode: 'create'; initialHost?: string }
  | { mode: 'edit'; rule: SiteRule };

export function RulesPage() {
  const sites = useSites();
  const createRule = useCreateRule();
  const patchRule = usePatchRule();
  const deleteRule = useDeleteRule();
  const [form, setForm] = useState<FormState>({ mode: 'closed' });

  const rules = sites.data?.rules ?? [];
  const ruleHosts = new Set(rules.map((rule) => rule.host));
  const hostsWithoutRule = (sites.data?.tracked_hosts ?? []).filter(
    (host) => !ruleHosts.has(host),
  );
  const busy = createRule.isPending || patchRule.isPending;

  const activeError =
    form.mode === 'create' ? createRule.error : patchRule.error;
  const fieldErrors =
    activeError instanceof ApiError ? activeError.fields : undefined;
  const bannerMessage =
    form.mode !== 'closed' && activeError !== null && fieldErrors === undefined
      ? (activeError as Error | null)?.message
      : deleteRule.error !== null
        ? deleteRule.error.message
        : undefined;

  const closeForm = () => {
    setForm({ mode: 'closed' });
    createRule.reset();
    patchRule.reset();
  };

  const submit = (body: SiteRuleNew | SiteRulePatch) => {
    if (form.mode === 'create') {
      createRule.mutate(body as SiteRuleNew, { onSuccess: closeForm });
    } else if (form.mode === 'edit') {
      if (form.rule.id !== undefined) {
        patchRule.mutate(
          { ruleID: form.rule.id, patch: body },
          { onSuccess: closeForm },
        );
      }
    }
  };

  return (
    <>
      <h1 className="nc-page-title">Site rules</h1>
      <p className="nc-page-caption nc-small">
        Rules teach NextChapter how to read a site&rsquo;s chapter URLs, so the
        extension can detect the series and chapter automatically. The extension
        can build simple rules for you; here you can edit the pattern directly.
      </p>

      {bannerMessage !== undefined && (
        <ErrorBanner>{bannerMessage}</ErrorBanner>
      )}

      {form.mode !== 'closed' ? (
        <section className="nc-section">
          <h2 className="nc-section-title">
            {form.mode === 'edit'
              ? `Edit rule — ${form.rule.host ?? ''}`
              : 'New rule'}
          </h2>
          <RuleForm
            key={
              form.mode === 'edit'
                ? form.rule.id
                : `create-${form.initialHost ?? ''}`
            }
            rule={form.mode === 'edit' ? form.rule : undefined}
            initialHost={form.mode === 'create' ? form.initialHost : undefined}
            fieldErrors={fieldErrors}
            busy={busy}
            onSubmit={submit}
            onCancel={closeForm}
          />
        </section>
      ) : (
        <section className="nc-section">
          <div className="nc-section-head">
            <h2 className="nc-section-title">Rules</h2>
            <button
              className="nc-btn-primary"
              type="button"
              onClick={() => {
                setForm({ mode: 'create' });
              }}
            >
              Add rule
            </button>
          </div>

          {!sites.isPending &&
          rules.length === 0 &&
          hostsWithoutRule.length === 0 ? (
            <p className="nc-entries-empty nc-small">
              No rules yet — add one here, or create one from the extension
              while on a chapter page.
            </p>
          ) : (
            <table className="nc-table">
              <thead>
                <tr>
                  <th scope="col">Host</th>
                  <th scope="col">Pattern</th>
                  <th scope="col" aria-label="Actions"></th>
                </tr>
              </thead>
              <tbody>
                {rules.map((rule) => (
                  <tr key={rule.id}>
                    <td className="nc-td-rule-host">{rule.host}</td>
                    <td>
                      <p
                        className="nc-rule-pattern"
                        title={rule.chapter_url_regex}
                      >
                        {rule.chapter_url_regex}
                      </p>
                      <p className="nc-rule-groups">
                        slug: <code>{rule.slug_capture_group}</code> · chapter:{' '}
                        <code>{rule.chapter_capture_group}</code>
                      </p>
                    </td>
                    <td className="nc-td-rule-actions">
                      <button
                        className="nc-row-action"
                        type="button"
                        onClick={() => {
                          setForm({ mode: 'edit', rule });
                        }}
                      >
                        Edit
                      </button>
                      <ConfirmInline
                        label="Delete"
                        question="Delete rule?"
                        busy={deleteRule.isPending}
                        onConfirm={() => {
                          if (rule.id !== undefined) deleteRule.mutate(rule.id);
                        }}
                      />
                    </td>
                  </tr>
                ))}
                {hostsWithoutRule.map((host) => (
                  <tr key={host}>
                    <td className="nc-td-norule-host">{host}</td>
                    <td>
                      <p className="nc-norule-hint nc-small">
                        No rule yet — captures here are manual.{' '}
                        <button
                          className="nc-btn-link"
                          type="button"
                          onClick={() => {
                            setForm({ mode: 'create', initialHost: host });
                          }}
                        >
                          Add one
                        </button>
                      </p>
                    </td>
                    <td className="nc-td-rule-actions"></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      )}
    </>
  );
}
