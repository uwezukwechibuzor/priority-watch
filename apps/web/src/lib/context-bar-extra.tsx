import {
  type Dispatch,
  type ReactNode,
  type SetStateAction,
  createContext,
  useContext,
  useState,
} from "react";

interface ContextBarExtraCtx {
  extraRight: ReactNode;
  setExtraRight: Dispatch<SetStateAction<ReactNode>>;
}

const ContextBarExtraContext = createContext<ContextBarExtraCtx>({
  extraRight: null,
  setExtraRight: () => undefined,
});

export function ContextBarExtraProvider({ children }: { children: ReactNode }) {
  const [extraRight, setExtraRight] = useState<ReactNode>(null);
  return (
    <ContextBarExtraContext value={{ extraRight, setExtraRight }}>
      {children}
    </ContextBarExtraContext>
  );
}

export function useContextBarExtra() {
  return useContext(ContextBarExtraContext);
}
