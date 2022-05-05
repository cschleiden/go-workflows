import {
  Button,
  Container,
  Form,
  FormControl,
  Nav,
  Navbar,
} from "react-bootstrap";
import { Outlet, useNavigate } from "react-router-dom";
import React, { useState } from "react";

import { LinkContainer } from "react-router-bootstrap";

function Layout() {
  const navigate = useNavigate();
  const [input, setInput] = useState("");
  const onGo = (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => {
    setInput("");
    navigate(`/${input}`);
  };

  return (
    <>
      <header>
        <Navbar bg="dark" variant="dark">
          <Container fluid>
            <Navbar.Brand href="/">go-workflows</Navbar.Brand>
            <Navbar.Toggle aria-controls="navbarScroll" />
            <Navbar.Collapse id="navbarScroll">
              <Nav
                className="me-auto my-2 my-lg-0"
                style={{ maxHeight: "100px" }}
                navbarScroll
              >
                <LinkContainer to="/">
                  <Nav.Link>List</Nav.Link>
                </LinkContainer>
              </Nav>
              <Form className="d-flex">
                <FormControl
                  type="search"
                  placeholder="InstanceID"
                  className="me-2"
                  aria-label="Search"
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                />
                <Button variant="outline-success" onClick={onGo}>
                  Go
                </Button>
              </Form>
            </Navbar.Collapse>
          </Container>
        </Navbar>
      </header>
      <main className="pt-5">
        <Container>
          <Outlet />
        </Container>
      </main>
    </>
  );
}

export default Layout;
