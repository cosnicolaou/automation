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

function loadStatusRecords(title, tag, endpoint, limit) {
    $.ajax({
        url: '/api/' + endpoint + '?' + $.param({ num: limit }),
        method: 'GET',
        success: function (data) {
            console.log(data);
            if (data.length > 0) {
                new gridjs.Grid({
                    title: title,
                    sort: true,
                    search: true,
                    columns: Object.keys(data[0]),
                    data: data,
                    pagination: {
                        limit: 100,
                    },
                    style: {
                        table: {
                            'white-space': 'nowrap'
                        }
                    },
                }).render(document.getElementById(tag));
            }
        }
    });
}

$('#reload-page').click(function (event) {
    event.preventDefault();
    location.reload();
    return false;
});

