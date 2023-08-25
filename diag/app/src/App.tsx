import { Route, Routes } from "react-router-dom";

import Home from "./Home";
import Instance from "./Instance";
import Layout from "./Layout";

function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Home />} />

        <Route path=":instanceId/:executionId" element={<Instance />} />
        <Route path=":instanceId" element={<Instance />} />
      </Route>
    </Routes>
  );
}

export default App;
