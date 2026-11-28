// Copy for the auto-accept switch, derived from the policy response.
import type { ProposalPolicyInfo } from "./api";

export function autoAcceptCanToggle(p: Pick<ProposalPolicyInfo, "stage"> | null): boolean {
    return p !== null && p.stage === "build";
}

export function autoAcceptStatus(p: ProposalPolicyInfo): string {
    const state = p.effective ? "Active" : "Not active";
    return p.reason ? `${state}: ${p.reason}` : state;
}

export function autoAcceptHint(p: ProposalPolicyInfo): string {
    if (autoAcceptCanToggle(p)) return "";
    return `Auto-accept can only be turned on in the build stage. The current stage is ${p.stage || "unknown"}.`;
}
