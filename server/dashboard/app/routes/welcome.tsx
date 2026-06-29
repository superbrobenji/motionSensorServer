import type { Route } from "./+types/welcome";
import { WelcomeComponent } from "../components/welcome/welcome";

export function meta(): Route.MetaDescriptors {
  return [
    { title: "New React Router App" },
    { name: "description", content: "Welcome to React Router!" },
  ];
}

export default function Welcome() {
  return <WelcomeComponent />;
}
