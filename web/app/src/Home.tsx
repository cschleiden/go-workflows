import React, { useEffect, useState } from "react";
import { Table } from "react-bootstrap";
import { Link } from "react-router-dom";

export default function () {
  const [instances, setInstances] = useState([]);

  useEffect(() => {
    (async () => {
      const baseUri = document.location.href;

      const res = await fetch(baseUri + "/api");
      const data = await res.json();
      setInstances(data);
    })();
  }, []);

  return (
    <div className="App">
      <header className="App-header">
        <h2>Instances</h2>
      </header>

      <Table striped bordered hover size="sm">
        <thead>
          <tr>
            <th>#</th>
            <th>Instance ID</th>
            <th>Execution ID</th>
          </tr>
        </thead>
        <tbody>
          {(instances || []).map((i, idx) => (
            <tr>
              <td>{idx}</td>
              <td>
                <Link to={`/${i["instance_id"]}`} key={i["instance_id"]}>
                  {i["instance_id"]}
                </Link>
              </td>
              <td>{i["execution_id"]}</td>
            </tr>
          ))}
        </tbody>
      </Table>
    </div>
  );
}
