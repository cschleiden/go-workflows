import React from "react";
import {
  Button,
  Container,
  Form,
  FormControl,
  Nav,
  Navbar,
} from "react-bootstrap";
import { LinkContainer } from "react-router-bootstrap";
import { Outlet } from "react-router-dom";

function Layout() {
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
                />
                <Button variant="outline-success">Go</Button>
              </Form>
            </Navbar.Collapse>
          </Container>
        </Navbar>
      </header>
      <main>
        <Container>
          <Outlet />
        </Container>
      </main>
    </>
  );
}

export default Layout;
