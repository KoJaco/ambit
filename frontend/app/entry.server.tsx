import type { EntryContext, RouterContextProvider } from "react-router";
import { ServerRouter } from "react-router";
import { renderToReadableStream } from "react-dom/server";

// Present so React Router does not install isbot. SPA mode does not use this
// for requests; ambit start serves the API and Vite serves the client.
export default async function handleRequest(
    request: Request,
    responseStatusCode: number,
    responseHeaders: Headers,
    routerContext: EntryContext,
    _loadContext: RouterContextProvider,
) {
    if (request.method.toUpperCase() === "HEAD") {
        return new Response(null, {
            status: responseStatusCode,
            headers: responseHeaders,
        });
    }
    const body = await renderToReadableStream(
        <ServerRouter context={routerContext} url={request.url} />,
    );
    await body.allReady;
    responseHeaders.set("Content-Type", "text/html");
    return new Response(body, {
        headers: responseHeaders,
        status: responseStatusCode,
    });
}
