import React from "react";
import { useParams } from "react-router-dom";

export default function () {
  let params = useParams();

  return (
    <div>
      <h2>Instance - {params.instanceId}</h2>
    </div>
  );
}
