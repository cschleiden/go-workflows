import { Route, Routes } from "react-router-dom";

import Home from "./Home";
import Instance from "./Instance";
import Layout from "./Layout";

function App() {
  return (
    <div>
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Home />} />

        <Route path=":instanceId/:executionId" element={<Instance />} />
      </Route>
    </Routes>
    </div>
  );
}

export default App;
