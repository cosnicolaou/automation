function runOp(dev, op, params) {
    let cgipars = params.map(function (value) {
        return { name: 'arg', value: value };
    });
    console.log('runOp', dev, op, cgipars);
    let cgiArgs = $.param([{ name: 'dev', value: dev }, { name: 'op', value: op }].concat(cgipars));
    $.ajax({
        url: '/api/operation?' + cgiArgs,
        method: 'GET',
        success: function (data) {
            $('#result').jsonViewer(data);
        },
        error: function (jqXHR, textStatus, errorThrown) {
            console.error('operation failed:', textStatus, errorThrown);
            $('#result').html('Error: ' + textStatus + ' - ' + errorThrown);
        }
    });
}
