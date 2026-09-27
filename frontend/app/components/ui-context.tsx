import React, { createContext, useContext, useState } from "react";

type UIContextValue = {
    pendingNodeId: string | null;
    setPendingNodeId: (id: string | null) => void;
    convertPending: (payload: { kind: string; title?: string }) => void;
    setConvertPending: (
        fn: (payload: { kind: string; title?: string }) => void
    ) => void;
};

const UIContext = createContext<UIContextValue | undefined>(undefined);

export function UIProvider({ children }: { children: React.ReactNode }) {
    const [pendingNodeId, setPendingNodeId] = useState<string | null>(null);
    const [convertPendingFn, setConvertPending] = useState<
        (payload: { kind: string; title?: string }) => void
    >(() => () => {});
    return (
        <UIContext.Provider
            value={{
                pendingNodeId,
                setPendingNodeId,
                convertPending: convertPendingFn,
                setConvertPending,
            }}
        >
            {children}
        </UIContext.Provider>
    );
}

export function useUIContext(): UIContextValue {
    const ctx = useContext(UIContext);
    if (!ctx) {
        // Fallback no-op context to avoid hydration/ordering issues
        return {
            pendingNodeId: null,
            setPendingNodeId: () => {},
            convertPending: () => {},
            setConvertPending: () => {},
        } as UIContextValue;
    }
    return ctx;
}
