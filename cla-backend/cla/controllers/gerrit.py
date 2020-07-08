# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Controller related to repository operations.
"""

import os
import uuid

import cla
import cla.hug_types
from cla.utils import get_project_instance, get_repository_instance, get_gerrit_instance
from cla.controllers.lf_group import LFGroup
from cla.models import DoesNotExist
from cla.models.dynamo_models import Gerrit

lf_group_client_url = os.environ.get('LF_GROUP_CLIENT_URL', '')
lf_group_client_id = os.environ.get('LF_GROUP_CLIENT_ID', '')
lf_group_client_secret = os.environ.get('LF_GROUP_CLIENT_SECRET', '')
lf_group_refresh_token = os.environ.get('LF_GROUP_REFRESH_TOKEN', '')
lf_group = LFGroup(lf_group_client_url, lf_group_client_id, lf_group_client_secret, lf_group_refresh_token)


def get_gerrit_by_project_id(project_id):
    gerrit = Gerrit()
    try:
        gerrits = gerrit.get_gerrit_by_project_id(project_id)
    except DoesNotExist:
        cla.log.warning('gerrit project id: {} does not exist'.format(project_id))
        return []
    except Exception as e:
        cla.log.warning('gerrit project id: {} does not exist, error: {}'.format(project_id, e))
        return {'errors': {f'a gerrit instance does not exist with the given project ID: {project_id}': str(e)}}

    if gerrits is None:
        cla.log.warning('gerrit project id: {} does not exist'.format(project_id))
        return []

    return [gerrit.to_dict() for gerrit in gerrits]


def get_gerrit(gerrit_id):
    gerrit = Gerrit()
    try:
        gerrit.load(str(gerrit_id))
    except DoesNotExist as err:
        cla.log.warning('a gerrit instance does not exist with the given Gerrit ID: {}'.format(gerrit_id))
        return {'errors': {'a gerrit instance does not exist with the given Gerrit ID. ': str(err)}}

    return gerrit.to_dict()


def create_gerrit(project_id,
                  gerrit_name,
                  gerrit_url,
                  group_id_icla,
                  group_id_ccla):
    """
    Creates a gerrit instance and returns the newly created gerrit object dict format.

    :param gerrit_project_id: The project ID of the gerrit instance
    :type gerrit_project_id: string
    :param gerrit_name: The new gerrit instance name
    :type gerrit_name: string
    :param gerrit_url: The new Gerrit URL.
    :type gerrit_url: string
    :param group_id_icla: The id of the LDAP group for ICLA. 
    :type group_id_icla: string
    :param group_id_ccla: The id of the LDAP group for CCLA. 
    :type group_id_ccla: string
    """

    gerrit = Gerrit()

    # Check if at least ICLA or CCLA is specified 
    if group_id_icla is None and group_id_ccla is None:
        cla.log.warning('Should specify at least a LDAP group for ICLA or CCLA.')
        return {'error': 'Should specify at least a LDAP group for ICLA or CCLA.'}

    # Check if ICLA exists
    if group_id_icla is not None:
        ldap_group_icla = lf_group.get_group(group_id_icla)
        if ldap_group_icla.get('error') is not None:
            cla.log.warning('The specified LDAP group for ICLA does not exist for project id: {}'
                            ', gerrit name: {} and group_id_icla: {}'.
                            format(project_id, gerrit_name, group_id_icla))
            return {'error_icla': 'The specified LDAP group for ICLA does not exist.'}

        gerrit.set_group_name_icla(ldap_group_icla.get('title'))
        gerrit.set_group_id_icla(str(group_id_icla))

    # Check if CCLA exists
    if group_id_ccla is not None:
        ldap_group_ccla = lf_group.get_group(group_id_ccla)
        if ldap_group_ccla.get('error') is not None:
            cla.log.warning('The specified LDAP group for CCLA does not exist for project id: {}'
                            ', gerrit name: {} and group_id_ccla: {}'.
                            format(project_id, gerrit_name, group_id_ccla))
            return {'error_ccla': 'The specified LDAP group for CCLA does not exist. '}

        gerrit.set_group_name_ccla(ldap_group_ccla.get('title'))
        gerrit.set_group_id_ccla(str(group_id_ccla))

    # Save Gerrit Instance
    gerrit.set_gerrit_id(str(uuid.uuid4()))
    gerrit.set_project_id(str(project_id))
    gerrit.set_gerrit_url(gerrit_url)
    gerrit.set_gerrit_name(gerrit_name)
    gerrit.save()
    cla.log.debug('saved gerrit instance with project id: {}, gerrit_name: {}'.
                  format(project_id, gerrit_name))

    return gerrit.to_dict()


def delete_gerrit(gerrit_id):
    """
    Deletes a gerrit instance

    :param gerrit_id: The ID of the gerrit instance.
    """
    gerrit = Gerrit()
    try:
        gerrit.load(str(gerrit_id))
    except DoesNotExist as err:
        cla.log.warning('a gerrit instance does not exist with the '
                        'given Gerrit ID: {} - unable to delete'.format(gerrit_id))
        return {'errors': {'gerrit_id': str(err)}}
    gerrit.delete()
    cla.log.debug('deleted gerrit instance with gerrit_id: {}'.format(gerrit_id))
    return {'success': True}


def get_agreement_html(gerrit_id, contract_type):
    console_v1_endpoint = cla.conf['CONTRIBUTOR_BASE_URL']
    console_v2_endpoint = cla.conf['CONTRIBUTOR_V2_BASE_URL']
    console_url = ''
    try:
        gerrit = get_gerrit_instance()
        gerrit.load(str(gerrit_id))
    except DoesNotExist as err:
        return {'errors': {'gerrit_id': str(err)}}
    try:
        project = get_project_instance()
        project.load(str(gerrit.get_project_id()))
    except DoesNotExist as err:
        return {'errors': {'project_id': str(err)}}


    # Temporary condition until all CLA Groups are ready for the v2 Contributor Console
    if project.get_version() == 'v2':
        # Generate url for the v2 console
        console_url = f'https://{console_v2_endpoint}/#/cla/gerrit/project/{gerrit.get_project_id()}/{contract_type}?redirect={gerrit.get_gerrit_url()}'
    else:
        # Generate url for the v1 contributor console
        console_url = f'https://{console_v1_endpoint}/#/cla/gerrit/project/{gerrit.get_project_id()}/{contract_type}'

    return f"""
        <html lang="en">
        <head>
        <!-- Required meta tags -->
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
        <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css" integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm" crossorigin="anonymous">
        <script src="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js" integrity="sha384-JZR6Spejh4U02d8jOt6vLEHfe/JQGiRRSQQxSfFWpi1MquVdAyjUar5+76PVCmYl" crossorigin="anonymous"></script>
        </head>
        <body style='margin-top:20;margin-left:0;margin-right:0;'>
            <div class="text-center">
                <img width=300px" src="https://cla-project-logo-prod.s3.amazonaws.com/lf-horizontal-color.svg" alt="community bridge logo"/>
            </div>
            <h2 class="text-center">EasyCLA Account Authorization</h2>
            <p class="text-center">
            Your account is not authorized under a signed CLA.  Click the button to authorize your account.
            </p>
            <p class="text-center">
            <a href="{console_url}" class="btn btn-primary" role="button">Proceed To Authorization</a>
            </p>
        </body>
        </html>
        """

