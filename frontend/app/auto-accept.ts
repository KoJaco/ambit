// Copy for the auto-accept switch, derived from the policy response.
import type { ProposalPolicyInfo } from "./api";

export function autoAcceptCanToggle(p: Pick<ProposalPolicyInfo, "stage"> | null): boolean {
    return p !== null && p.stage === "build";
}

// An empty stage means the method defines no stages, so there is no build stage to be in.
function stageSentence(stage: string): string {
    return stage ? `The current stage is ${stage}.` : "This method has no stages.";
}

// Status line. Built from the policy fields, not the server's `reason`, which names the raw
// config key and quotes an empty stage.
export function autoAcceptStatus(p: ProposalPolicyInfo): string {
    if (p.effective) return "Active";
    if (!p.auto_accept) return "Not active: Auto-accept is off";
    return `Not active: ${stageSentence(p.stage)} Auto-accept works only in the build stage.`;
}

export function autoAcceptHint(p: ProposalPolicyInfo): string {
    if (autoAcceptCanToggle(p)) return "";
    return `Auto-accept can only be turned on in the build stage. ${stageSentence(p.stage)}`;
}
