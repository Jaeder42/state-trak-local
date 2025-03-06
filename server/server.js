const express = require("express");
const cors = require("cors");
const app = express();
const port = 3001;
app.use(cors());
app.get("/", (req, res) => {
  res.send("Hello World!");
});

app.get("/:id", (req, res) => {
  const { id } = req.params;
  console.log("test");
  const data = require(`./data/output/${id}.json`);
  res.send(data);
});

app.listen(port, () => {
  console.log(`Example app listening on port ${port}`);
});
