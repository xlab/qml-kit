# Add more folders to ship with the application, here
folder_01.source = qml/
folder_01.target = qml
DEPLOYMENTFOLDERS = folder_01

QT += widgets

# Additional import path used to resolve QML modules in Creator's code model
QML_IMPORT_PATH =

SOURCES += main.cpp

include(qtquick2applicationviewer/qtquick2applicationviewer.pri)
qtcAddDeployment()

OTHER_FILES += \
    qml/Ctrl.qml
