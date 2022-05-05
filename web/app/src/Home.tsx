import { Link, useLocation } from "react-router-dom";
import { Pagination, Table } from "react-bootstrap";

import { LinkContainer } from "react-router-bootstrap";
import React from "react";
import { WorkflowInstanceRef } from "./client";
import useFetch from "react-fetch-hook";

function useQuery() {
  const { search } = useLocation();

  return React.useMemo(() => new URLSearchParams(search), [search]);
}

function Home() {
  const count = 20;

  const query = useQuery();
  const afterId = query.get("after");
  const previousId = query.get("previous");

  const { isLoading, data, error } = useFetch<WorkflowInstanceRef[]>(
    `/api?count=${count}` + (afterId ? `&after=${afterId || previousId}` : "")
  );

  return (
    <div className="App">
      <header className="App-header">
        <h2>Instances</h2>
      </header>

      {isLoading && <div>Loading...</div>}

      {!isLoading && (
        <>
          <Table striped bordered hover size="sm">
            <thead>
              <tr>
                <th>#</th>
                <th>Instance ID</th>
                <th>Execution ID</th>
                <th>Created At</th>
              </tr>
            </thead>
            <tbody>
              {(data || []).map((i, idx) => (
                <tr key={i.instance.instance_id}>
                  <td>{idx}</td>
                  <td>
                    <Link
                      to={`/${i.instance.instance_id}`}
                      key={i.instance.instance_id}
                    >
                      {i.instance.instance_id}
                    </Link>
                  </td>
                  <td>{i.instance.execution_id}</td>
                  <td>{i.created_at}</td>
                </tr>
              ))}
            </tbody>
          </Table>

          <div className="d-flex justify-content-center">
            <Pagination>
              <LinkContainer to={`/?previous=${previousId || ""}`}>
                <Pagination.Prev disabled={!previousId && !afterId} />
              </LinkContainer>
              <LinkContainer
                to={`/?previous=${afterId || ""}&after=${
                  (data && data[data.length - 1].instance.instance_id) || ""
                }`}
              >
                <Pagination.Next disabled={!data || data.length < count} />
              </LinkContainer>
            </Pagination>
          </div>
        </>
      )}
    </div>
  );
}

export default Home;
