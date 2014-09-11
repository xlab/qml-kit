import QtQuick 2.2
import QtQuick.Controls 1.1
// TODO: bump versions to the latest

ApplicationWindow {
    minimumWidth: 600; minimumHeight: 400

    MouseArea {
        anchors.fill: parent
        onClicked: Qt.quit()

        Image {
            anchors.fill: parent
            fillMode: Image.Tile
            source: "image://images/background.png"
        }

        Text {
            text: "Hello, world!"
            anchors.centerIn: parent
            font.pointSize: 72
            color: "#b10000"
        }
    }
}
