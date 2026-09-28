import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
    index("routes/home.tsx"),
    route("relationship/:relationshipId", "routes/relationship.tsx"),
    route("node/:nodeId", "routes/node.tsx"),
] satisfies RouteConfig;
