import React from "react";
import ReactDOM from "react-dom/client";

import { App } from "./app/App";
import { AppProviders } from "./app/providers/AppProviders";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("app") as HTMLElement).render(
  <React.StrictMode>
    <AppProviders>
      <App />
    </AppProviders>
  </React.StrictMode>,
);
