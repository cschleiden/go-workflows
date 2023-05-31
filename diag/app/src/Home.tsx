import { Pagination, Table } from "react-bootstrap";
import { Link, useLocation } from "react-router-dom";

import React from "react";
import useFetch from "react-fetch-hook";
import { LinkContainer } from "react-router-bootstrap";
import { WorkflowInstanceRef } from "./client";

function useQuery() {
  const { search } = useLocation();

  return React.useMemo(() => new URLSearchParams(search), [search]);
}

function Home() {
  const count = 20;

  const query = useQuery();
  const afterId = query.get("after");
  const page = +(query.get("page") || 1);

  const { isLoading, data, error } = useFetch<WorkflowInstanceRef[]>(
    document.location.pathname +
      `api/?count=${count}` +
      (afterId ? `&after=${afterId}` : "")
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
                <th>Instance ID</th>
                <th>Execution ID</th>
                <th>Parent Instance ID</th>
                <th>Created At</th>
              </tr>
            </thead>
            <tbody>
              {(data || []).map((i) => (
                <tr key={i.instance.instance_id}>
                  <td>
                    <Link to={`/${i.instance.instance_id}`}>
                      <code>{i.instance.instance_id}</code>
                    </Link>
                  </td>
                  <td>
                    <code>{i.instance.execution_id}</code>
                  </td>
                  <td>
                    <Link to={`/${i.instance.parent_instance}`}>
                      <code>{i.instance.parent_instance}</code>
                    </Link>
                  </td>
                  <td>
                    <code>{i.created_at}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>

          <div className="d-flex justify-content-center">
            <Pagination>
              <LinkContainer to="/?">
                <Pagination.First disabled={!afterId} />
              </LinkContainer>
              <Pagination.Item active>{page}</Pagination.Item>
              <LinkContainer
                to={`/?after=${
                  (data && data[data.length - 1].instance.instance_id) || ""
                }&page=${page + 1}`}
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
