export type ScopePreview = {
    globs: string[];
    defaultsToImplementation: boolean;
    protected: boolean;
};

function filled(globs: string[]): string[] {
    return globs.map((glob) => glob.trim()).filter((glob) => glob !== "");
}

// Preview of which paths a scope covers. Empty scope uses implementation.
// Protected is reported separately and does not change the globs.
export function previewScope(input: {
    implementation: string[];
    scope: string[];
    protected: boolean;
}): ScopePreview {
    const scope = filled(input.scope);
    const implementation = filled(input.implementation);
    const defaultsToImplementation = scope.length === 0;
    return {
        globs: defaultsToImplementation ? implementation : scope,
        defaultsToImplementation,
        protected: input.protected,
    };
}
