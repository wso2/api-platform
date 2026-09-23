/**
 * Works out which translator applies to one provider on a proxy, and says so in
 * one voice.
 *
 * Four screens show this: the create page, the Providers tab, the definition
 * tab and the policies tab. They must agree — the same provider on the same
 * proxy cannot read as configured on one screen and unconfigured on another — so
 * the decision is made here once and the screens only render what they are
 * given.
 *
 * The wording lives here too, for the same reason. A screen that composes its
 * own sentence is a screen that will eventually word it differently.
 */

import type { ProxyProviderTransformer, SelectablePolicy } from './types';

/**
 * What applies to a provider, and why.
 *
 * - `none`     the provider already speaks the proxy's own format, so nothing
 *              translates
 * - `auto`     a translator was matched from the two formats and is not yet
 *              recorded on the proxy
 * - `manual`   a translator the proxy already names
 * - `unresolved` translation is needed but nothing provides it
 *
 * `auto` and `manual` read identically on screen. They differ only in whether
 * the proxy already carries the translator, which is what decides whether a
 * write has to add it — and once it has been added, nothing distinguishes a
 * translator that was matched from one that was picked. Saying which it was
 * would be a guess.
 * - `invalid`  a translator was chosen but is no longer in the catalogue
 * - `unknown`  the catalogue has not loaded, so no judgement is possible yet
 */
export type TransformerResolutionStatus =
  | 'none'
  | 'auto'
  | 'manual'
  | 'unresolved'
  | 'invalid'
  | 'unknown';

export interface TransformerResolution {
  status: TransformerResolutionStatus;
  /** The policy that applies, when one does. */
  policy?: SelectablePolicy;
  /**
   * What to call this on screen — the policy's display name where there is a
   * policy, the reassurance where none is needed, and the problem where there
   * is one.
   */
  title?: string;
  /**
   * The line under the title: the policy's identifier and how it came to be
   * chosen. Absent where there is no policy to identify.
   */
  detail?: string;
  /**
   * Something true about this translator beyond which one it is — that it is
   * attached where nothing needs translating, say. Absent when there is
   * nothing to add.
   */
  note?: string;
  /** How this reads: settled, fine as it is, or a problem. */
  tone?: 'success' | 'warning' | 'info';
}

export interface TransformerResolutionInput {
  /** The format the proxy accepts from clients. */
  inboundTemplate?: string;
  /** The format this provider speaks. */
  providerTemplate?: string;
  /** A translator chosen explicitly for this provider, if any. */
  chosenTransformer?: ProxyProviderTransformer | null;
  /**
   * Whether the absence of one is itself a decision.
   *
   * Nothing attached is otherwise ambiguous — a provider that has not been
   * looked at yet and one whose translator was just taken off both hold
   * nothing — and only the second must stop being offered a match it has
   * already refused.
   */
  hasNoTransformer?: boolean;
  /** The policies available to choose from. */
  policies?: SelectablePolicy[];
  /** Whether the catalogue has finished loading. */
  policiesLoaded?: boolean;
  /** The interface's display name, for wording that names it. */
  interfaceLabel?: string;
  /** The provider's display name, for wording that names it. */
  providerLabel?: string;
}

/**
 * The policy name that translates between two formats.
 *
 * Built from template handles on both sides, never from display names: the
 * handle is what the catalogue names its policies after, and a display name can
 * be edited without the policy being renamed.
 */
export const autoMatchedPolicyName = (
  inboundTemplate: string,
  providerTemplate: string
): string => `${inboundTemplate}-to-${providerTemplate}-transformer`;

const findPolicy = (
  policies: SelectablePolicy[] | undefined,
  name: string
): SelectablePolicy | undefined =>
  policies?.find((policy) => policy.name === name);

/**
 * Whether a provider needs translating at all.
 *
 * Matched on handles, so a provider whose format is the proxy's own format needs
 * nothing. An absent inbound interface means the proxy takes its format from its
 * primary provider, which is the same thing for that provider.
 */
const speaksTheSameFormat = (
  inboundTemplate?: string,
  providerTemplate?: string
): boolean => {
  if (!inboundTemplate || !providerTemplate) {
    return false;
  }
  return inboundTemplate === providerTemplate;
};

/**
 * Decides what applies to one provider.
 *
 * Order matters. A provider that needs no translation is settled before the
 * catalogue is consulted, so a slow or failed catalogue cannot make a proxy that
 * needs nothing look like a proxy with a problem.
 */
export const resolveTransformer = (
  input: TransformerResolutionInput
): TransformerResolution => {
  const {
    inboundTemplate,
    providerTemplate,
    chosenTransformer,
    hasNoTransformer = false,
    policies,
    policiesLoaded = true,
    interfaceLabel,
    providerLabel,
  } = input;

  // Nothing has been chosen yet, so there is nothing to resolve. Saying "no
  // transformer configured" against an empty provider slot reports a problem
  // the user has not had the chance to cause.
  if (!providerTemplate) {
    return { status: 'none' };
  }

  const forInterface = interfaceLabel ?? inboundTemplate;
  const needsNothing = speaksTheSameFormat(inboundTemplate, providerTemplate);

  // What is attached is settled before what would be needed. A translator on
  // the proxy runs whether or not the formats have since come to match, so
  // reporting "no transformer needed" over one that is attached would describe
  // a proxy that does not exist — and would hide the only thing that could be
  // removed.
  if (chosenTransformer?.type) {
    if (!policiesLoaded) {
      return { status: 'unknown', title: 'Checking available transformers…' };
    }
    const chosen = findPolicy(policies, chosenTransformer.type);
    if (!chosen) {
      // Naming the direction the stored policy actually covers is the only way
      // a reader can tell why it stopped applying — the policy is still real,
      // it just no longer translates between these two formats.
      return {
        status: 'invalid',
        title: 'Transformer no longer valid',
        detail: `${chosenTransformer.type} does not translate to the ${forInterface ?? 'current'} interface.`,
        tone: 'warning',
      };
    }
    return {
      status: 'manual',
      policy: chosen,
      title: chosen.displayName,
      detail: chosen.name,
      ...(needsNothing
        ? {
            note: `Not needed — ${providerLabel ?? 'this provider'} already speaks ${interfaceLabel ?? 'this format'}`,
          }
        : {}),
      tone: 'success',
    };
  }

  if (needsNothing) {
    // Said plainly rather than left blank: a reader who sees a transformer
    // section on one provider and nothing on another cannot tell whether the
    // second is fine or still loading. Naming both formats answers that.
    return {
      status: 'none',
      title: 'No transformer needed',
      detail: `${providerLabel ?? 'This provider'} already speaks ${interfaceLabel ?? 'the same format'}.`,
      tone: 'info',
    };
  }

  // Nothing chosen: the catalogue decides, so it has to have loaded. Reporting
  // this as unconfigured would blame the proxy for a failed request.
  if (!policiesLoaded) {
    return { status: 'unknown', title: 'Checking available transformers…' };
  }

  if (inboundTemplate && providerTemplate && !hasNoTransformer) {
    const matched = findPolicy(
      policies,
      autoMatchedPolicyName(inboundTemplate, providerTemplate)
    );
    if (matched) {
      return {
        status: 'auto',
        policy: matched,
        title: matched.displayName,
        detail: matched.name,
        tone: 'success',
      };
    }
  }

  // Reported on this provider's own card and nowhere else. It does not mark the
  // row, does not summarise at page level, and does not gate saving or
  // deploying — the consequence is deferred to runtime, where invoking this
  // provider fails and the others carry on.
  return {
    status: 'unresolved',
    title: 'No transformer configured',
    // Says what failed and what to do about it. "Not configured" alone leaves a
    // reader to work out both which direction is unsupported and where a policy
    // could come from.
    detail: `Routing ${forInterface ?? 'inbound'}-format requests to ${
      providerLabel ?? 'this provider'
    } can't be auto-matched. Pick a policy from the Policy Hub or one already deployed on your gateway.`,
    tone: 'warning',
  };
};

/** Whether this resolution renders anything at all. */
export const hasTransformerMessage = (
  resolution: TransformerResolution
): boolean => resolution.status !== 'none';
