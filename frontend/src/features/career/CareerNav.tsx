import { AppNavigation } from "../../shared/AppNavigation";

export function CareerNav({ active }: { active: "discovery" | "studio" }): React.JSX.Element {
  return <AppNavigation active={active} />;
}
