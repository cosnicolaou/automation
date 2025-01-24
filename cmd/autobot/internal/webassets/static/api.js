function runOperation(dev, op, params) {
    $("#resultBar").empty();
    let cgipars = params.map(function (value) {
        return { name: 'arg', value: value };
    });
    console.log('runOp', dev, op, cgipars);
    let cgiArgs = $.param([{ name: 'dev', value: dev }, { name: 'op', value: op }].concat(cgipars));
    $.ajax({
        url: '/api/operation?' + cgiArgs,
        method: 'GET',
        success: function (data) {
            $('#resultBar').jsonViewer(data, { collapsed: false });
        },
        error: function (jqXHR, textStatus, errorThrown) {
            console.error('operation failed:', textStatus, errorThrown);
            console.log(jqXHR);
            $('#resultBar').jsonViewer({ "Error": jqXHR.responseText, "Status": textStatus, "ErrorThrown": errorThrown });
        },
    });
}

function runCondition(dev, op, params) {
    $("#resultBar").empty();
    let cgipars = params.map(function (value) {
        return { name: 'arg', value: value };
    });
    console.log('runOp', dev, op, cgipars);
    let cgiArgs = $.param([{ name: 'dev', value: dev }, { name: 'op', value: op }].concat(cgipars));
    $.ajax({
        url: '/api/condition?' + cgiArgs,
        method: 'GET',
        success: function (data) {
            $('#resultBar').jsonViewer(data);
        },
        error: function (jqXHR, textStatus, errorThrown) {
            console.error('operation failed:', textStatus, errorThrown);
            console.log(jqXHR);
            $('#resultBar').jsonViewer({ "Error": jqXHR.responseText, "Status": textStatus, "ErrorThrown": errorThrown });
        },
    });
}

$(document).ready(function () {
    $("#reload-config").on("click", function (event) {
        event.preventDefault(); // Prevent the default link behavior
        $("#resultBar").empty();
        console.log('reload');
        $.ajax({
            url: '/api/reload?',
            method: 'GET',
            success: function (data) {
                $('#resultBar').jsonViewer(data);
                window.location.reload(true);
            },
            error: function (jqXHR, textStatus, errorThrown) {
                console.error('operation failed:', textStatus, errorThrown);
                console.log(jqXHR);
                $('#resultBar').jsonViewer({ "Error": jqXHR.responseText, "Status": textStatus, "ErrorThrown": errorThrown });
            },
        });
    });
});