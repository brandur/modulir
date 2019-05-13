// TODO: Reconnect a closed connection.
console.log("Connecting to Modulir /websocket");
var socket = new WebSocket("ws://localhost:{{.Port}}/websocket");

socket.onclose = function(event) {
  console.log("Lost webhook connection");
}

socket.onmessage = function (event) {
  console.log(`Received event of type '${event.type}' data: ${event.data}`);

  var data = JSON.parse(event.data);

  switch(data.type) {
    case "build_complete":
      // 1000 = "Normal closure" and the second parameter is a human-readable
      // reason.
      socket.close(1000, "Reloading page");

      console.log("Reloading page");
      location.reload(true);

      break;

    default:
      console.log(`Don't know how to handle type '${data.type}'`);
  }
}
