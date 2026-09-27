import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
    index("routes/home.tsx"),
    route("node/:nodeId", "routes/home.tsx"),
] satisfies RouteConfig;
