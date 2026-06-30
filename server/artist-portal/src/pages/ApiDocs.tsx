import SwaggerUI from "swagger-ui-react";
import "swagger-ui-react/swagger-ui.css";

export function ApiDocs() {
  return (
    <div className="api-docs">
      <h1>API Reference</h1>
      <SwaggerUI url="/openapi/v1.yaml" />
    </div>
  );
}
